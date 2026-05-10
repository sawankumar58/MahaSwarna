package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gorilla/websocket"
	"github.com/robfig/cron/v3"

	"github.com/mahaswarna/pricing/application"
	"github.com/mahaswarna/pricing/events"
	pricinghttp "github.com/mahaswarna/pricing/http"
	"github.com/mahaswarna/pricing/infrastructure"
	"github.com/mahaswarna/pricing/jobs"
	"github.com/mahaswarna/pricing/watchdog"
	pricingws "github.com/mahaswarna/pricing/ws"

	contractsevents "github.com/mahaswarna/contracts/events"
	infraredis "github.com/mahaswarna/infrastructure/redis"
	"github.com/mahaswarna/infrastructure/pgnotify"
	"github.com/mahaswarna/observability"
)

func main() {
	mustEnv(
		"DATABASE_URL",
		"REDIS_SENTINEL_1", "REDIS_SENTINEL_2", "REDIS_SENTINEL_3",
		"GEMINI_API_KEY",
		"JWT_PUBLIC_KEY",
		"INTERNAL_JWT_SECRET",
	)

	if dsn := os.Getenv("SENTRY_DSN"); dsn != "" {
		sentry.Init(sentry.ClientOptions{Dsn: dsn})
		defer sentry.Flush(2 * time.Second)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Infrastructure ────────────────────────────────────────────────────────

	db, err := infrastructure.NewDB(ctx)
	if err != nil {
		slog.Error("postgres failed", "err", err)
		os.Exit(1)
	}

	rdb := infraredis.NewFailoverClient()

	// ── Repositories ──────────────────────────────────────────────────────────

	snapRepo := infrastructure.NewAIRateSnapshotRepository(db)
	ratesRepo := infrastructure.NewRatesRepository(db)
	cache := infrastructure.NewRateCache(rdb)
	_ = ratesRepo // available for manual override reads if needed

	gemini, err := infrastructure.NewGeminiClient(ctx)
	if err != nil {
		slog.Error("gemini client failed", "err", err)
		os.Exit(1)
	}

	// ── Events ────────────────────────────────────────────────────────────────

	notifier := events.NewNotifier(db)

	// ── WebSocket layer ───────────────────────────────────────────────────────

	registry := pricingws.NewConnectionRegistry()
	fanout := pricingws.NewBufferedFanout(registry)
	banSvc := pricingws.NewBanService(registry)

	wsServer, err := pricingws.NewServer(registry, fanout, rdb)
	if err != nil {
		slog.Error("ws server init failed", "err", err)
		os.Exit(1)
	}

	// ── Use cases ─────────────────────────────────────────────────────────────

	// FlagsRepository is the real DB-backed implementation for reading/writing
	// feature flags. Required for the OQ-8 kill-switch escalation path in
	// generate_ai_rates_usecase.go — without this, SetFlag is a no-op and the
	// kill-switch never activates after 3 consecutive Gemini full-run failures.
	flagsRepo := infrastructure.NewFlagsRepository(db)

	getRateUC := application.NewGetRateUseCase(cache, snapRepo)
	getHistoryUC := application.NewGetHistoryUseCase(snapRepo)
	generateUC := application.NewGenerateAIRatesUseCase(
		gemini, snapRepo, cache, notifier, flagsRepo, flagsRepo,
	)

	// ── Warmup Redis cache from DB on startup ─────────────────────────────────

	go func() {
		snaps, err := snapRepo.GetLatestAll(ctx)
		if err != nil {
			slog.Warn("startup: cache warmup failed", "err", err)
			return
		}
		if err := cache.WarmAll(ctx, snaps); err != nil {
			slog.Warn("startup: cache warmup redis error", "err", err)
			return
		}
		slog.Info("startup: redis cache warmed", "cities", len(snaps))
	}()

	// ── Observability ─────────────────────────────────────────────────────────

	health := observability.NewHealthChecker()
	health.Register("postgres", func(c context.Context) error { return db.Ping(c) })
	health.Register("redis", func(c context.Context) error { return rdb.Ping(c).Err() })

	// ── pg NOTIFY listeners ───────────────────────────────────────────────────

	// Wire the fanout to listen on ai_rate_snapshot_ready.
	listeners := events.NewListeners(db, rdb, banSvc, registry)
	go fanout.Run(ctx)

	// RegisterListener wires BufferedFanout.HandleNotification to the
	// ai_rate_snapshot_ready pg NOTIFY channel so rate updates reach WS clients.
	// THIS CALL IS MANDATORY — without it, fanout.pending is always empty.
	// See P0 fix: github.com/mahaswarna/pricing verification report.
	fanoutListener := pgnotify.NewListener(db,
		[]string{contractsevents.ChannelAIRateSnapshotReady},
		nil, // no catch-up needed for fanout — events are ephemeral
	)
	fanoutListener.On(contractsevents.ChannelAIRateSnapshotReady, fanout.HandleNotification)
	go fanoutListener.Listen(ctx)

	// Start pg NOTIFY listener (blocks until ctx is cancelled, reconnects on error).
	go listeners.Start(ctx)

	// Block until the startup catch-up completes so /health/ready returns 503 until ready.
	<-listeners.Ready()
	slog.Info("pricing: listeners ready")

	// ── Watchdog ──────────────────────────────────────────────────────────────

	wd := watchdog.NewRateQualityWatchdog(snapRepo, cache, notifier, flagsAdapter)
	go wd.Run(ctx)

	// ── Cron ─────────────────────────────────────────────────────────────────

	// CRITICAL: initialise cron with IST. robfig/cron defaults to UTC.
	istLoc, _ := time.LoadLocation("Asia/Kolkata")
	c := cron.New(cron.WithLocation(istLoc))
	jobs.NewAIRateSchedulerJob(generateUC).Register(c)
	c.Start()

	// ── HTTP server ───────────────────────────────────────────────────────────

	ratesH := pricinghttp.NewRatesHandler(getRateUC, getHistoryUC)
	router := pricinghttp.NewRouter(ratesH, wsServer, health, rdb)

	srv := &http.Server{
		Addr:         ":4002",
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second, // longer to allow WS upgrade
	}

	go func() {
		slog.Info("pricing service listening", "addr", ":4002")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	// ARCHITECTURE NOTE (from ARCHITECTURE.md §main.go):
	// Phase 1: stop accepting new WS upgrades (15s grace).
	// Phase 2: send close frame to all live WS connections.
	// docker-compose.prod.yml must set stop_grace_period: 20s for pricing.

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	slog.Info("pricing: shutting down — draining connections")

	// Phase 1: stop accepting new WS upgrades.
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	srv.Shutdown(shutCtx)

	// Phase 2: send close frame to all live WS connections.
	registry.CloseAll(websocket.CloseGoingAway, "server restarting")

	c.Stop()
	slog.Info("pricing service stopped")
}

// ── helpers ───────────────────────────────────────────────────────────────────

func mustEnv(keys ...string) {
	for _, k := range keys {
		if os.Getenv(k) == "" {
			slog.Error("required env missing", "key", k)
			os.Exit(1)
		}
	}
}
