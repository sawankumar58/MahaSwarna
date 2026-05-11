package application

import (
	"context"
	"time"

	"github.com/mahaswarna/core/domain"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/redis/go-redis/v9"
)

type EvaluateThresholdsUseCase struct {
	alerts  *infrastructure.AlertsRepository
	rates   *infrastructure.RateProjection
	deliver *DeliverAlertUseCase
	rdb     *redis.Client
}

func NewEvaluateThresholdsUseCase(a *infrastructure.AlertsRepository, r *infrastructure.RateProjection,
	d *DeliverAlertUseCase, rdb *redis.Client) *EvaluateThresholdsUseCase {
	return &EvaluateThresholdsUseCase{alerts: a, rates: r, deliver: d, rdb: rdb}
}

// ActivePairs returns every distinct (city_id, metal) pair that has at least one
// pending alert. AlertThresholdJob calls this to drive its evaluation loop so it
// never relies on a hardcoded city list.
func (uc *EvaluateThresholdsUseCase) ActivePairs(ctx context.Context) ([]infrastructure.CityMetalPair, error) {
	return uc.alerts.ListActiveCityMetalPairs(ctx)
}

func (uc *EvaluateThresholdsUseCase) Evaluate(ctx context.Context, cityID, metal string) error {
	snap, err := uc.rates.GetLatestRate(ctx, cityID)
	if err != nil { return nil }
	rate := snap.Gold
	if metal == domain.MetalSilver { rate = snap.Silver }
	alerts, err := uc.alerts.ListPendingByCityMetal(ctx, cityID, metal)
	if err != nil { return err }
	for _, a := range alerts {
		key := "alert_debounce:" + a.ID.String()
		set, _ := uc.rdb.SetNX(ctx, key, "1", time.Hour).Result()
		if !set { continue }
		if (a.Direction == domain.DirectionAbove && rate >= a.Threshold) ||
			(a.Direction == domain.DirectionBelow && rate <= a.Threshold) {
			uc.deliver.Deliver(ctx, a, rate)
		}
	}
	return nil
}
