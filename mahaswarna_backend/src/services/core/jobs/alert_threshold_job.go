package jobs

import (
	"context"
	"log/slog"

	"github.com/robfig/cron/v3"
	"github.com/mahaswarna/core/application"
)

type AlertThresholdJob struct{ usecase *application.EvaluateThresholdsUseCase }

func NewAlertThresholdJob(uc *application.EvaluateThresholdsUseCase) *AlertThresholdJob {
	return &AlertThresholdJob{usecase: uc}
}

func (j *AlertThresholdJob) Register(c *cron.Cron) {
	c.AddFunc("* * * * *", j.run)
}

func (j *AlertThresholdJob) run() {
	ctx := context.Background()

	pairs, err := j.usecase.ActivePairs(ctx)
	if err != nil {
		slog.Error("alert_threshold_job: failed to fetch active city/metal pairs", "err", err)
		return
	}
	if len(pairs) == 0 {
		return // no pending alerts — nothing to evaluate
	}

	for _, p := range pairs {
		if err := j.usecase.Evaluate(ctx, p.CityID, p.Metal); err != nil {
			slog.Error("alert_threshold_job: eval error", "city", p.CityID, "metal", p.Metal, "err", err)
		}
	}
}
