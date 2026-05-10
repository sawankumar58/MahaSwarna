package bff

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/mahaswarna/gateway/lib"
	"github.com/mahaswarna/gateway/middleware"
	contractshttp "github.com/mahaswarna/contracts/http"
	"github.com/mahaswarna/shared"
	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"
)

const (
	// Two-key Redis cache split (ARCHITECTURE.md §Gateway):
	//   home:shared:{cityID}  — rates + AI rate + flags; shared across all city users
	//   home:alerts:{userID}  — active alerts; per-user
	bffSharedCachePrefix = "home:shared"
	bffAlertsCachePrefix = "home:alerts"

	// bffCacheTTL is the TTL for both cache keys (30 s per architecture spec).
	bffCacheTTL = 30 * time.Second

	// bffUpstreamTimeout is the per-upstream context deadline (800 ms per spec).
	// Target: total BFF response < 1,500 ms.
	bffUpstreamTimeout = 800 * time.Millisecond
)

// sharedHomeData is the city-scoped, user-agnostic portion of the BFF response.
// Stored under home:shared:{cityID} so all users in the same city share one copy.
type sharedHomeData struct {
	Rate   *contractshttp.RateResponse        `json:"rate"`
	AIRate *contractshttp.AIRateResponse      `json:"ai_rate"`
	Flags  *contractshttp.FeatureFlagsResponse `json:"flags"`
}

// HomeAggregator fans out to pricing, core, and intelligence in parallel to
// assemble the BFF home screen response in a single gateway round-trip.
//
// Redis cache strategy (two-key split):
//   - City-shared key: rates + AI rate + flags (identical for all users in a city)
//   - Per-user key:    active alerts (user-specific)
//
// Degraded mode: if any upstream call fails, the response is still returned
// with available fields populated and _degraded=true so the client can show
// partial data rather than an error screen.
type HomeAggregator struct {
	coreURL         string
	pricingURL      string
	intelligenceURL string
	coreBreaker     *gobreaker.CircuitBreaker
	pricingBreaker  *gobreaker.CircuitBreaker
	intelBreaker    *gobreaker.CircuitBreaker
	sharedCache     *lib.FallbackCache
	alertsCache     *lib.FallbackCache
	client          *http.Client
}

// NewHomeAggregator creates a HomeAggregator with the given upstream URLs and breakers.
func NewHomeAggregator(
	coreURL, pricingURL, intelligenceURL string,
	coreBreaker, pricingBreaker, intelligenceBreaker *gobreaker.CircuitBreaker,
	rdb *redis.Client,
) *HomeAggregator {
	return &HomeAggregator{
		coreURL:         coreURL,
		pricingURL:      pricingURL,
		intelligenceURL: intelligenceURL,
		coreBreaker:     coreBreaker,
		pricingBreaker:  pricingBreaker,
		intelBreaker:    intelligenceBreaker,
		sharedCache:     lib.NewFallbackCache(rdb, bffCacheTTL),
		alertsCache:     lib.NewFallbackCache(rdb, bffCacheTTL),
		client: &http.Client{
			Transport: lib.SharedTransport(), // shared pool — no duplicate connections
			Timeout:   bffUpstreamTimeout,
		},
	}
}

// Handle is the http.HandlerFunc for GET /v1/bff/home.
func (a *HomeAggregator) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromCtx(ctx)
	cityID := r.URL.Query().Get("cityId")
	if cityID == "" {
		cityID = "mumbai" // default city
	}

	sharedKey := fmt.Sprintf("%s:%s", bffSharedCachePrefix, cityID)
	alertsKey := fmt.Sprintf("%s:%s", bffAlertsCachePrefix, userID)

	// Serve stale cache immediately on open pricing circuit.
	if a.pricingBreaker.State() == gobreaker.StateOpen {
		shared.Logger.Warn("bff: pricing breaker open, serving stale home")
		if resp, ok := a.serveStale(ctx, sharedKey, alertsKey); ok {
			writeJSON(w, resp)
			return
		}
	}

	resp := a.aggregate(ctx, r, cityID)

	writeJSON(w, apiSuccess(resp))

	// Cache only non-degraded responses to avoid poisoning with partial data.
	// Store the two keys independently so a per-user alerts miss doesn't
	// invalidate the city-shared rate cache.
	if !resp.Degraded {
		homeShared := sharedHomeData{Rate: resp.Rate, AIRate: resp.AIRate, Flags: resp.Flags}
		if sharedBytes, err := json.Marshal(homeShared); err == nil {
			go a.sharedCache.Store(context.Background(), sharedKey, sharedBytes)
		}
		if alertsBytes, err := json.Marshal(resp.Alerts); err == nil {
			go a.alertsCache.Store(context.Background(), alertsKey, alertsBytes)
		}
	}
}

// serveStale assembles a stale BFF response from the two Redis cache keys.
// Returns (response, true) if at least the shared (rate) key is present.
// If only the shared key exists, alerts default to nil and Degraded is set.
func (a *HomeAggregator) serveStale(ctx context.Context, sharedKey, alertsKey string) (map[string]any, bool) {
	sharedBytes, err := a.sharedCache.Get(ctx, sharedKey)
	if err != nil || len(sharedBytes) == 0 {
		return nil, false // no stale rate data — cannot serve home screen
	}

	var sd sharedHomeData
	if err := json.Unmarshal(sharedBytes, &sd); err != nil {
		return nil, false
	}

	var alerts []contractshttp.AlertResponse
	if alertsBytes, aerr := a.alertsCache.Get(ctx, alertsKey); aerr == nil && len(alertsBytes) > 0 {
		_ = json.Unmarshal(alertsBytes, &alerts)
	}

	resp := contractshttp.BFFHomeResponse{
		Rate:     sd.Rate,
		AIRate:   sd.AIRate,
		Flags:    sd.Flags,
		Alerts:   alerts,
		Degraded: true, // stale data is always flagged as degraded
	}
	return apiSuccess(resp), true
}

// aggregate fans out to all three upstreams concurrently.
func (a *HomeAggregator) aggregate(ctx context.Context, r *http.Request, cityID string) contractshttp.BFFHomeResponse {
	type result[T any] struct {
		data T
		err  error
	}

	var (
		rateCh   = make(chan result[*contractshttp.RateResponse], 1)
		aiCh     = make(chan result[*contractshttp.AIRateResponse], 1)
		flagsCh  = make(chan result[*contractshttp.FeatureFlagsResponse], 1)
		alertsCh = make(chan result[[]contractshttp.AlertResponse], 1)
	)

	hdrs := extractForwardHeaders(r)

	var wg sync.WaitGroup
	wg.Add(4)

	// 1. Live rate (pricing service)
	go func() {
		defer wg.Done()
		rate, err := fetchJSON[contractshttp.RateResponse](ctx, a.client, a.pricingBreaker,
			fmt.Sprintf("%s/internal/rates?cityId=%s", a.pricingURL, cityID), hdrs)
		rateCh <- result[*contractshttp.RateResponse]{data: rate, err: err}
	}()

	// 2. AI rate snapshot (pricing service)
	go func() {
		defer wg.Done()
		aiRate, err := fetchJSON[contractshttp.AIRateResponse](ctx, a.client, a.pricingBreaker,
			fmt.Sprintf("%s/internal/rates/ai?cityId=%s", a.pricingURL, cityID), hdrs)
		aiCh <- result[*contractshttp.AIRateResponse]{data: aiRate, err: err}
	}()

	// 3. Feature flags — re-use from context (already loaded by FeatureFlags middleware).
	// No network call needed; avoids an extra round-trip within the 800 ms budget.
	go func() {
		defer wg.Done()
		flags := middleware.FlagsFromCtx(ctx)
		flagsCh <- result[*contractshttp.FeatureFlagsResponse]{data: flags}
	}()

	// 4. Active alerts for this user (core service)
	go func() {
		defer wg.Done()
		type alertList struct {
			Alerts []contractshttp.AlertResponse `json:"alerts"`
		}
		list, err := fetchJSON[alertList](ctx, a.client, a.coreBreaker,
			fmt.Sprintf("%s/internal/alerts", a.coreURL), hdrs)
		var alerts []contractshttp.AlertResponse
		if list != nil {
			alerts = list.Alerts
		}
		alertsCh <- result[[]contractshttp.AlertResponse]{data: alerts, err: err}
	}()

	wg.Wait()

	rateRes := <-rateCh
	aiRes := <-aiCh
	flagsRes := <-flagsCh
	alertsRes := <-alertsCh

	degraded := rateRes.err != nil || aiRes.err != nil || alertsRes.err != nil

	if rateRes.err != nil {
		shared.Logger.Warn("bff: rate fetch failed", "err", rateRes.err)
	}
	if aiRes.err != nil {
		shared.Logger.Warn("bff: ai rate fetch failed", "err", aiRes.err)
	}
	if alertsRes.err != nil {
		shared.Logger.Warn("bff: alerts fetch failed", "err", alertsRes.err)
	}

	return contractshttp.BFFHomeResponse{
		Rate:     rateRes.data,
		AIRate:   aiRes.data,
		Flags:    flagsRes.data,
		Alerts:   alertsRes.data,
		Degraded: degraded,
	}
}

// fetchJSON makes a GET request through the circuit breaker, with exponential-backoff
// retry on 502/503/504, and unmarshals the response body into T.
// Returns nil, err on any failure.
func fetchJSON[T any](ctx context.Context, client *http.Client, cb *gobreaker.CircuitBreaker, url string, headers http.Header) (*T, error) {
	var result *T

	_, err := cb.Execute(func() (any, error) {
		// L-1: wrap in DoWithRetry so transient upstream 5xx are retried before
		// counting as a circuit-breaker failure.
		resp, err := lib.DoWithRetry(ctx, lib.DefaultRetryConfig, func() (*http.Response, error) {
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if reqErr != nil {
				return nil, reqErr
			}
			for k, vals := range headers {
				for _, v := range vals {
					req.Header.Add(k, v)
				}
			}
			return client.Do(req)
		})
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("upstream %s returned %d", url, resp.StatusCode)
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB limit
		if err != nil {
			return nil, err
		}

		// Unwrap the APIResponse envelope.
		var envelope struct {
			OK   bool `json:"ok"`
			Data *T   `json:"data"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, err
		}
		if !envelope.OK || envelope.Data == nil {
			return nil, fmt.Errorf("upstream %s returned ok=false", url)
		}

		result = envelope.Data
		return result, nil
	})

	return result, err
}

// extractForwardHeaders builds the header set to propagate to upstreams.
func extractForwardHeaders(r *http.Request) http.Header {
	fwd := make(http.Header)
	for _, h := range []string{
		"X-User-ID", "X-User-Tier",
		"X-Service-Token", "X-Service-Timestamp",
		"X-Request-ID", "Traceparent",
	} {
		if v := r.Header.Get(h); v != "" {
			fwd.Set(h, v)
		}
	}
	return fwd
}

// writeJSON writes a JSON response with Content-Type and 200 status.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	body, err := json.Marshal(v)
	if err != nil {
		shared.Logger.Error("bff: marshal error", "err", err)
		http.Error(w, `{"ok":false}`, http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(body)
}

// apiSuccess constructs a canonical APIResponse envelope.
func apiSuccess[T any](data T) map[string]any {
	return map[string]any{"ok": true, "data": data}
}
