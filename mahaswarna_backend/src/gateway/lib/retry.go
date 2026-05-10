package lib

import (
	"context"
	"math"
	"net/http"
	"time"

	"github.com/mahaswarna/shared"
)

// RetryConfig controls retry behaviour for upstream calls.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	// RetryOn is the set of HTTP status codes that trigger a retry.
	// Defaults to {502, 503, 504} if nil.
	RetryOn []int
}

// DefaultRetryConfig is suitable for all upstream calls from the gateway.
var DefaultRetryConfig = RetryConfig{
	MaxAttempts: 3,
	BaseDelay:   100 * time.Millisecond,
	MaxDelay:    2 * time.Second,
	RetryOn:     []int{502, 503, 504},
}

// DoWithRetry executes fn up to cfg.MaxAttempts times, backing off
// exponentially between attempts.
//
// fn should return (statusCode, error). A non-retryable status code (e.g. 200,
// 400, 401) stops the loop immediately. Retryable status codes and non-nil
// errors trigger the next attempt.
//
// The returned values are from the final attempt.
func DoWithRetry(ctx context.Context, cfg RetryConfig, fn func() (*http.Response, error)) (*http.Response, error) {
	retryOn := cfg.RetryOn
	if retryOn == nil {
		retryOn = DefaultRetryConfig.RetryOn
	}

	var lastResp *http.Response
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err := fn()
		lastResp = resp
		lastErr = err

		if err == nil && resp != nil && !isRetryableStatus(resp.StatusCode, retryOn) {
			return resp, nil
		}

		if attempt < cfg.MaxAttempts-1 {
			delay := backoff(cfg.BaseDelay, cfg.MaxDelay, attempt)
			shared.Logger.Debug("retrying upstream call",
				"attempt", attempt+1,
				"delay_ms", delay.Milliseconds(),
				"err", err,
			)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return lastResp, lastErr
}

// backoff computes exponential delay with full jitter.
func backoff(base, max time.Duration, attempt int) time.Duration {
	exp := base * time.Duration(math.Pow(2, float64(attempt)))
	if exp > max {
		exp = max
	}
	return exp
}

func isRetryableStatus(code int, retryOn []int) bool {
	for _, c := range retryOn {
		if code == c {
			return true
		}
	}
	return false
}
