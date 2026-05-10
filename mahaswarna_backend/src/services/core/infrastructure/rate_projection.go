package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RateSnapshot struct {
	Gold   float64 `json:"gold"`
	Silver float64 `json:"silver"`
	CityID string  `json:"city_id"`
	Stale  bool    `json:"stale"`
}

type RateProjection struct{ rdb *redis.Client }

func NewRateProjection(rdb *redis.Client) *RateProjection { return &RateProjection{rdb: rdb} }

func (p *RateProjection) GetLatestRate(ctx context.Context, cityID string) (*RateSnapshot, error) {
	data, err := p.rdb.Get(ctx, fmt.Sprintf("rate:latest:ai:%s", cityID)).Bytes()
	if err != nil { return nil, fmt.Errorf("rate miss %s: %w", cityID, err) }
	var snap RateSnapshot
	if err := json.Unmarshal(data, &snap); err != nil { return nil, err }
	return &snap, nil
}
