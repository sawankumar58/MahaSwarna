package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	infraredis "github.com/mahaswarna/infrastructure/redis"
	"github.com/mahaswarna/shared"
)

const (
	defaultPort         = "4000"
	readHeaderTimeout   = 5 * time.Second
	readTimeout         = 15 * time.Second
	writeTimeout        = 30 * time.Second
	idleTimeout         = 120 * time.Second
	shutdownGracePeriod = 20 * time.Second
)

func main() {
	// ── Structured logger ─────────────────────────────────────────────────────
	logLevel := slog.LevelInfo
	if os.Getenv("APP_ENV") == "development" {
		logLevel = slog.LevelDebug
	}
	shared.Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	log := shared.Logger

	// ── Validate required environment ─────────────────────────────────────────
	required := []string{
		"INTERNAL_JWT_SECRET",    // HMAC key for service-to-service tokens
		"JWT_PUBLIC_KEY",         // RSA public key (PEM) for verifying user JWTs (RS256)
		"CORE_BASE_URL",
		"PRICING_BASE_URL",
		"INTELLIGENCE_BASE_URL",
		"REDIS_SENTINEL_1",
		"REDIS_SENTINEL_2",
		"REDIS_SENTINEL_3",
	}
	for _, k := range required {
		if os.Getenv(k) == "" {
			log.Error("missing required environment variable", "key", k)
			os.Exit(1)
		}
	}

	// ── Redis (Sentinel) ──────────────────────────────────────────────────────
	rdb := infraredis.NewFailoverClient()
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Error("redis ping failed", "err", err)
		os.Exit(1)
	}
	log.Info("redis sentinel connected")

	// ── HTTP router ───────────────────────────────────────────────────────────
	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = defaultPort
	}

	router := buildRouter(rdb)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("gateway listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("shutdown signal received, draining…")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
	} else {
		log.Info("gateway stopped cleanly")
	}
}
