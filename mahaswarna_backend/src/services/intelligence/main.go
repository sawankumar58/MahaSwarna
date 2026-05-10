package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	ce "github.com/mahaswarna/contracts/events"
	"github.com/mahaswarna/infrastructure/pgnotify"
	infraRedis "github.com/mahaswarna/infrastructure/redis"
	"github.com/mahaswarna/infrastructure/postgres"
	"github.com/mahaswarna/intelligence/application"
	"github.com/mahaswarna/intelligence/events"
	handler "github.com/mahaswarna/intelligence/http"
	"github.com/mahaswarna/intelligence/infrastructure"
	"github.com/mahaswarna/intelligence/jobs"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// ── Infrastructure ────────────────────────────────────────────────────────
	pool, err := postgres.NewPool(ctx, "intelligence")
	must(err, "postgres pool")

	rdb := infraRedis.NewFailoverClient()

	s3, err := infrastructure.NewS3Client(ctx)
	must(err, "s3 client")

	cdn, err := infrastructure.NewCDNURLBuilder()
	must(err, "cdn url builder")

	moderation, err := infrastructure.NewModerationClient(ctx)
	must(err, "moderation client")
	defer moderation.Close()

	pricingClient := infrastructure.NewPricingClient(mustEnv("PRICING_SERVICE_URL"))

	// ── Migrations ────────────────────────────────────────────────────────────
	if err := infrastructure.RunMigrations(ctx, pool); err != nil {
		must(err, "migrations")
	}

	// ── Repositories ─────────────────────────────────────────────────────────
	shopRepo := infrastructure.NewShopRepository(pool)
	designRepo := infrastructure.NewDesignRepository(pool)
	invoiceRepo := infrastructure.NewInvoiceRepository(pool)
	subProj := infrastructure.NewSubscriptionProjection(rdb)
	viewCache := infrastructure.NewViewCountCache(rdb)
	pdfBuilder := infrastructure.NewInvoicePDFBuilder()

	// ── Use cases ─────────────────────────────────────────────────────────────
	registerShop := application.NewRegisterShopUseCase(shopRepo, subProj)
	getBannerURL := application.NewGetBannerUploadURLUseCase(shopRepo, s3, subProj)
	confirmBanner := application.NewConfirmBannerUploadUseCase(shopRepo, s3, moderation, cdn)
	searchDesign := application.NewSearchDesignUseCase(designRepo, viewCache)
	recommendDesign := application.NewRecommendDesignUseCase(designRepo)
	generateInvoice := application.NewGenerateInvoiceUseCase(shopRepo, invoiceRepo, pdfBuilder, subProj, rdb)

	// ── HTTP handlers ─────────────────────────────────────────────────────────
	pubKey := mustLoadPublicKey(mustEnv("JWT_PUBLIC_KEY_PATH"))
	shopH := handler.NewShopHandler(registerShop, getBannerURL, confirmBanner, shopRepo)
	catalogH := handler.NewCatalogHandler(searchDesign, recommendDesign)
	invoiceH := handler.NewInvoiceHandler(generateInvoice, invoiceRepo, shopRepo, pricingClient)
	router := handler.NewRouter(pubKey, rdb, shopH, catalogH, invoiceH)

	// ── Event listeners ───────────────────────────────────────────────────────
	listener := pgnotify.NewListener(pool, []string{
		ce.ChannelSubscriptionActivated,
		ce.ChannelSubscriptionExpired,
		ce.ChannelAccountDeleted,
	}, func(reconnectCtx context.Context) error {
		// NOTIFY reconnect invariant: re-populate the subscription projection for
		// all active shops to recover any events missed during the reconnect window.
		slog.Info("pgnotify reconnected — re-seeding subscription projection")
		return infrastructure.RepopulateSubscriptionProjection(reconnectCtx, subProj, shopRepo)
	})

	events.RegisterListeners(listener, subProj, shopRepo, invoiceRepo, s3)
	go listener.Listen(ctx)

	// ── Background jobs ───────────────────────────────────────────────────────
	flushJob := jobs.NewFlushViewCountsJob(viewCache, designRepo)
	c := cron.New()
	if _, err := c.AddFunc("@every 5m", flushJob.Run); err != nil {
		must(err, "cron add flush job")
	}
	c.Start()
	defer c.Stop()

	// ── HTTP server ───────────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second, // PDFs can be slow to generate
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("intelligence service listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()

	slog.Info("shutting down intelligence service")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}

	// Final view count flush before exit.
	flushJob.Run()
	slog.Info("intelligence service stopped")
}

func mustLoadPublicKey(path string) *rsa.PublicKey {
	b, err := os.ReadFile(path)
	must(err, "read jwt public key")
	block, _ := pem.Decode(b)
	if block == nil {
		panic("invalid PEM in JWT_PUBLIC_KEY_PATH")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	must(err, "parse jwt public key")
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		panic("JWT_PUBLIC_KEY_PATH is not an RSA public key")
	}
	return rsaPub
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required env var %q is not set", key))
	}
	return v
}

func must(err error, msg string) {
	if err != nil {
		slog.Error(msg, "err", err)
		os.Exit(1)
	}
}
