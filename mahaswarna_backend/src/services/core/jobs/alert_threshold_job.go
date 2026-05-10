package jobs

import (
	"context"
	"log/slog"

	"github.com/robfig/cron/v3"
	"github.com/mahaswarna/core/application"
)

type AlertThresholdJob struct{ usecase *application.EvaluateThresholdsUseCase }

func NewAlertThresholdJob(uc *application.EvaluateThresholdsUseCase) *AlertThresholdJob { return &AlertThresholdJob{usecase: uc} }

func (j *AlertThresholdJob) Register(c *cron.Cron) {
	c.AddFunc("* * * * *", func() {
		ctx := context.Background()
		// TODO: query distinct (city_id, metal) from active alerts instead of hardcoded list.
		for _, city := range []string{"mumbai", "delhi", "bangalore", "hyderabad", "chennai"} {
			for _, metal := range []string{"gold", "silver"} {
				if err := j.usecase.Evaluate(ctx, city, metal); err != nil {
					slog.Error("alert eval error", "city", city, "metal", metal, "err", err)
				}
			}
		}
	})
}
