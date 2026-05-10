package jobs

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/mahaswarna/pricing/application"
)

// AIRateSchedulerJob runs the Gemini rate fetch on the IST market-hours schedule.
//
// ARCHITECTURE INVARIANT: robfig/cron defaults to UTC. The cron runner MUST be
// initialised with IST explicitly in main.go:
//
//	istLoc, _ := time.LoadLocation("Asia/Kolkata")
//	c := cron.New(cron.WithLocation(istLoc))
//
// Without this, "0 10-19 * * 1-6" fires at 10:00–19:00 UTC = 15:30–00:30 IST — wrong.
// The IST hour guard below is defense-in-depth only, not the primary gate.
type AIRateSchedulerJob struct {
	uc  *application.GenerateAIRatesUseCase
	mu  sync.Mutex // prevents overlapping runs on the single VPS
}

func NewAIRateSchedulerJob(uc *application.GenerateAIRatesUseCase) *AIRateSchedulerJob {
	return &AIRateSchedulerJob{uc: uc}
}

// Register adds the job to a cron instance.
// Schedule: top of every hour from 10:00–19:00 IST, Mon–Sat.
// cron.WithLocation(istLoc) MUST be set on c — verified by main.go.
func (j *AIRateSchedulerJob) Register(c *cron.Cron) {
	c.AddFunc("0 10-19 * * 1-6", j.run)
}

func (j *AIRateSchedulerJob) run() {
	// IST hour guard — defense-in-depth against cron timezone drift.
	// Primary gate is the cron expression + WithLocation(istLoc).
	// Cron "0 10-19 * * 1-6" fires at hours 10, 11, … 19 (inclusive).
	// Guard must allow h == 19 and block h >= 20, so use h > 19 (not h >= 20).
	istLoc, _ := time.LoadLocation("Asia/Kolkata")
	now := time.Now().In(istLoc)
	h := now.Hour()
	if h < 10 || h > 19 {
		slog.Debug("ai_rate_scheduler: outside IST window, skipping", "hour_ist", h)
		return
	}

	// sync.Mutex guard — single VPS, single cron instance.
	// At ~50k DAU when a second pricing node is introduced: replace with a Redis
	// distributed lock to prevent double-run across nodes.
	if !j.mu.TryLock() {
		slog.Warn("ai_rate_scheduler: previous run still in progress, skipping")
		return
	}
	defer j.mu.Unlock()

	slog.Info("ai_rate_scheduler: starting run", "ist_time", now.Format("15:04"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := j.uc.Execute(ctx); err != nil {
		slog.Error("ai_rate_scheduler: run failed", "err", err)
		return
	}

	slog.Info("ai_rate_scheduler: run complete", "duration", time.Since(now))
}
