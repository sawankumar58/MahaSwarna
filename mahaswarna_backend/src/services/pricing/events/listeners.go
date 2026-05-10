package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	contractsevents "github.com/mahaswarna/contracts/events"
	"github.com/mahaswarna/infrastructure/pgnotify"
	"github.com/mahaswarna/pricing/ws"
	"github.com/redis/go-redis/v9"
)

// Listeners subscribes to pg NOTIFY channels relevant to the pricing service.
// ARCHITECTURE INVARIANT: RebuildSubscriptionProjectionViaAPI is called at startup
// AND on every pg reconnect to recover events lost during the gap.
type Listeners struct {
	db      *pgxpool.Pool
	rdb     *redis.Client
	banSvc  *ws.BanService
	registry *ws.ConnectionRegistry // for alert_delivered → WS push
	ready   chan struct{}
	readyMu sync.Once
}

func NewListeners(db *pgxpool.Pool, rdb *redis.Client, banSvc *ws.BanService, registry *ws.ConnectionRegistry) *Listeners {
	return &Listeners{
		db:       db,
		rdb:      rdb,
		banSvc:   banSvc,
		registry: registry,
		ready:    make(chan struct{}),
	}
}

// Ready returns a channel that is closed once the startup catch-up completes.
// main.go blocks on <-listeners.Ready() before starting the HTTP server so that
// /health/ready returns 503 until the projection is current.
func (l *Listeners) Ready() <-chan struct{} {
	return l.ready
}

// Start begins listening. Call in a goroutine: go listeners.Start(ctx).
func (l *Listeners) Start(ctx context.Context) {
	channels := []string{
		contractsevents.ChannelUserBanned,
		contractsevents.ChannelSubscriptionActivated,
		contractsevents.ChannelSubscriptionExpired,
		contractsevents.ChannelAlertDelivered,
	}
	listener := pgnotify.NewListener(l.db, channels,
		func(ctx context.Context) error {
			return l.rebuildProjectionWithRetry(ctx)
		},
	)

	listener.On(contractsevents.ChannelUserBanned, l.handleUserBanned)
	listener.On(contractsevents.ChannelSubscriptionActivated, l.handleSubscriptionActivated)
	listener.On(contractsevents.ChannelSubscriptionExpired, l.handleSubscriptionExpired)
	listener.On(contractsevents.ChannelAlertDelivered, l.handleAlertDelivered)

	go listener.Listen(ctx)
}

// handleUserBanned force-disconnects a banned user from any open WebSocket connections.
func (l *Listeners) handleUserBanned(_ context.Context, _, payload string) {
	var p contractsevents.UserBannedPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		slog.Warn("events: malformed user_banned payload", "err", err)
		return
	}
	slog.Info("events: user banned — disconnecting WS", "user_id", p.UserID)
	l.banSvc.Disconnect(p.UserID)
}

// handleSubscriptionActivated updates the Redis subscription projection so the
// rate alert evaluator immediately applies the new tier without waiting for a restart.
// CROSS-SCHEMA NOTE: pricing MUST NOT query core's subscriptions table directly.
func (l *Listeners) handleSubscriptionActivated(ctx context.Context, _, payload string) {
	var p contractsevents.SubscriptionChangedPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		slog.Warn("events: malformed subscription_activated payload", "err", err)
		return
	}
	slog.Info("events: subscription activated — refreshing projection", "user_id", p.UserID, "tier", p.Tier)
	if err := l.rdb.Set(ctx, "sub:"+p.UserID, p.Tier, 24*time.Hour).Err(); err != nil {
		slog.Warn("events: subscription_activated redis set failed", "user_id", p.UserID, "err", err)
	}
}

// handleSubscriptionExpired downgrades the user's cached tier immediately.
func (l *Listeners) handleSubscriptionExpired(ctx context.Context, _, payload string) {
	var p contractsevents.SubscriptionExpiredPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		slog.Warn("events: malformed subscription_expired payload", "err", err)
		return
	}
	slog.Info("events: subscription expired — downgrading projection", "user_id", p.UserID)
	// Downgrade to FREE tier. Deleting the key would cause a miss on next check;
	// setting to "FREE" is explicit and avoids an extra API call to core.
	if err := l.rdb.Set(ctx, "sub:"+p.UserID, "FREE", 24*time.Hour).Err(); err != nil {
		slog.Warn("events: subscription_expired redis set failed", "user_id", p.UserID, "err", err)
	}
}

// handleAlertDelivered pushes the alert notification to the user's live WS connection.
// This is a targeted unicast: only the user who owns the alert receives the message.
// If the user has no active WS connection the message is silently dropped — FCM
// already delivered the push notification via core's deliver_alert_usecase.go.
func (l *Listeners) handleAlertDelivered(_ context.Context, _, payload string) {
	var p contractsevents.AlertDeliveredPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		slog.Warn("events: malformed alert_delivered payload", "err", err)
		return
	}

	msg := ws.OutboundAlertMessage{
		Channel: ws.ChannelAlerts,
		AlertID: p.AlertID,
		CityID:  p.CityID,
		Metal:   p.Metal,
		Rate:    p.Rate,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		slog.Warn("events: alert_delivered marshal failed", "alert_id", p.AlertID, "err", err)
		return
	}
	l.registry.Send(p.UserID, b)
	slog.Debug("events: alert_delivered pushed to WS", "user_id", p.UserID, "alert_id", p.AlertID)
}

// rebuildProjectionWithRetry calls core's internal subscriptions API with exponential
// backoff. Core may still be starting when pricing initialises.
//
// RETRY POLICY (from ARCHITECTURE.md §events/listeners.go):
// 8 attempts: 1s → 2s → 4s → 8s → 16s → 32s → 64s → 128s (total ~255s ≈ 4.25 min).
// On exhaustion: log SEV-2 to Sentry; /health/ready stays 503.
func (l *Listeners) rebuildProjectionWithRetry(ctx context.Context) error {
	coreURL := os.Getenv("CORE_INTERNAL_URL")
	if coreURL == "" {
		coreURL = "http://core:4001"
	}

	delays := []time.Duration{1, 2, 4, 8, 16, 32, 64, 128}
	var lastErr error

	for i, delay := range delays {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := l.fetchSubscriptionProjection(ctx, coreURL)
		if err == nil {
			slog.Info("events: subscription projection rebuilt", "attempt", i+1)
			// Signal readiness on the first successful rebuild.
			l.readyMu.Do(func() { close(l.ready) })
			return nil
		}

		lastErr = err
		slog.Warn("events: projection rebuild failed, retrying",
			"attempt", i+1, "delay", delay, "err", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay * time.Second):
		}
	}

	slog.Error("events: projection rebuild exhausted all retries", "err", lastErr)
	// Do NOT close ready — /health/ready returns 503 to block gateway routing.
	return lastErr
}

// fetchSubscriptionProjection calls GET /internal/subscriptions/active on core service
// and warms the local Redis projection used by the rate alert evaluator.
// CROSS-SCHEMA NOTE: pricing MUST NOT directly query core's subscriptions table.
func (l *Listeners) fetchSubscriptionProjection(ctx context.Context, coreURL string) error {
	url := coreURL + "/internal/subscriptions/active"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	// Service-to-service auth (HMAC-SHA256).
	ts := time.Now().Unix()
	token, tsStr := serviceTokenHeader(ts)
	req.Header.Set("X-Service-Token", token)
	req.Header.Set("X-Service-Timestamp", tsStr)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("core subscriptions returned %d", resp.StatusCode)
	}

	// Decode and warm Redis projection.
	var subs []struct {
		UserID string `json:"userId"`
		Tier   string `json:"tier"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&subs); err != nil {
		return fmt.Errorf("decode subscriptions: %w", err)
	}

	pipe := l.rdb.Pipeline()
	for _, s := range subs {
		pipe.Set(ctx, "sub:"+s.UserID, s.Tier, 24*time.Hour)
	}
	_, err = pipe.Exec(ctx)
	return err
}
