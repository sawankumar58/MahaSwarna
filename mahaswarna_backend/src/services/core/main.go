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
	"github.com/mahaswarna/core/application"
	"github.com/mahaswarna/core/events"
	corehttp "github.com/mahaswarna/core/http"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/mahaswarna/core/jobs"
	infraredis "github.com/mahaswarna/infrastructure/redis"
	goredis "github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

func main() {
	mustEnv(
		"DATABASE_URL", "REDIS_SENTINEL_1", "REDIS_SENTINEL_2", "REDIS_SENTINEL_3",
		"JWT_PRIVATE_KEY", "JWT_PUBLIC_KEY", "INTERNAL_JWT_SECRET",
		"FIREBASE_SERVICE_ACCOUNT_JSON", "GOOGLE_SERVICE_ACCOUNT_JSON",
		"PLAY_INTEGRITY_DECRYPTION_KEY",
	)

	if dsn := os.Getenv("SENTRY_DSN"); dsn != "" {
		sentry.Init(sentry.ClientOptions{Dsn: dsn})
		defer sentry.Flush(2 * time.Second)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := infrastructure.NewDB(ctx)
	if err != nil { slog.Error("postgres failed", "err", err); os.Exit(1) }

	rdb := infraredis.NewFailoverClient()
	var _ *goredis.Client = rdb // type assertion

	// Repos.
	userRepo    := infrastructure.NewUserRepository(db)
	sessionRepo := infrastructure.NewSessionRepository(db)
	consentRepo := infrastructure.NewConsentLogRepository(db)
	subRepo     := infrastructure.NewSubscriptionRepository(db)
	receiptRepo := infrastructure.NewReceiptLogRepository(db)
	alertsRepo  := infrastructure.NewAlertsRepository(db)
	tokenRepo   := infrastructure.NewDeviceTokenRepository(db)
	flagRepo    := infrastructure.NewFlagsRepository(db, rdb)
	auditRepo   := infrastructure.NewAuditLogRepository(db)
	rateProj    := infrastructure.NewRateProjection(rdb)

	notifier := events.NewNotifier(db)

	// OTP.
	firebaseProv, err := infrastructure.NewFirebaseOtpProvider(ctx)
	if err != nil { slog.Error("firebase failed", "err", err); os.Exit(1) }
	msg91Prov := infrastructure.NewMsg91OtpProvider()
	otpProv   := infrastructure.NewOtpProvider(envOrDefault("OTP_PROVIDER", "both"), firebaseProv, msg91Prov)

	playClient, err := infrastructure.NewGooglePlayClient(ctx)
	if err != nil { slog.Error("play client failed", "err", err); os.Exit(1) }

	fcmClient, err := infrastructure.NewPushNotificationClient(ctx)
	if err != nil { slog.Error("fcm failed", "err", err); os.Exit(1) }

	// Use cases.
	loginUC   := mustLogin(userRepo, sessionRepo, otpProv, rdb, auditRepo)
	otpUC     := application.NewOTPSendUseCase(otpProv, rdb, flagRepo)
	refreshUC := application.NewRefreshUseCase(userRepo, sessionRepo, loginUC)
	logoutUC  := application.NewLogoutUseCase(sessionRepo)
	deleteUC  := application.NewDeleteAccountUseCase(userRepo, sessionRepo, auditRepo, notifier)
	consentUC := application.NewLogConsentUseCase(consentRepo)
	verifyUC  := application.NewVerifyReceiptUseCase(receiptRepo, subRepo, userRepo, playClient, auditRepo, notifier)
	restoreUC := application.NewRestoreSubscriptionUseCase(subRepo, playClient)
	tokenUC      := application.NewRegisterDeviceTokenUseCase(tokenRepo)
	deregTokenUC := application.NewDeregisterDeviceTokenUseCase(tokenRepo)
	deliverUC := application.NewDeliverAlertUseCase(alertsRepo, tokenRepo, fcmClient, auditRepo, notifier)
	evalUC    := application.NewEvaluateThresholdsUseCase(alertsRepo, rateProj, deliverUC, rdb)

	// Handlers.
	authH     := corehttp.NewAuthHandler(otpUC, loginUC, refreshUC, logoutUC)
	compH     := corehttp.NewComplianceHandler(deleteUC, consentUC)
	billingH  := corehttp.NewBillingHandler(verifyUC, restoreUC)
	alertsH   := corehttp.NewAlertsHandler(alertsRepo)
	tokenH    := corehttp.NewDeviceTokenHandler(tokenUC, deregTokenUC)
	flagsH    := corehttp.NewFlagsHandler(flagRepo)
	internalH := corehttp.NewInternalHandler(subRepo)

	router := corehttp.NewRouter(authH, compH, billingH, alertsH, tokenH, flagsH, internalH, rdb)

	// pg NOTIFY listeners — MUST complete startup catch-up before /health/ready returns 200.
	listeners := events.NewListeners(db, rdb, subRepo, userRepo, flagRepo)
	go listeners.Start(ctx)
	<-listeners.Ready()

	// Cron — IST timezone (critical: robfig/cron defaults to UTC).
	istLoc, _ := time.LoadLocation("Asia/Kolkata")
	c := cron.New(cron.WithLocation(istLoc))
	jobs.NewAlertThresholdJob(evalUC).Register(c)
	jobs.NewSubscriptionExpiryJob(subRepo, notifier).Register(c)
	jobs.NewHardDeleteJob(userRepo, auditRepo, notifier).Register(c)
	c.Start()

	srv := &http.Server{Addr: ":4001", Handler: router, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second}
	go func() {
		slog.Info("core service listening", "addr", ":4001")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err); os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	srv.Shutdown(shutCtx)
	c.Stop()
	slog.Info("core service stopped")
}

func mustLogin(users *infrastructure.UserRepository, sessions *infrastructure.SessionRepository,
	otp infrastructure.OtpProvider, rdb *goredis.Client, audit *infrastructure.AuditLogRepository) *application.LoginUseCase {
	uc, err := application.NewLoginUseCase(users, sessions, otp, rdb, audit)
	if err != nil { slog.Error("login usecase failed", "err", err); os.Exit(1) }
	return uc
}

func mustEnv(keys ...string) {
	for _, k := range keys {
		if os.Getenv(k) == "" { slog.Error("required env missing", "key", k); os.Exit(1) }
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}
