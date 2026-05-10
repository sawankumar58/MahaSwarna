package infrastructure

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/core/domain"
	contractshttp "github.com/mahaswarna/contracts/http"
	"github.com/redis/go-redis/v9"
)

const (flagsCacheKey = "feature_flags:public"; flagsCacheTTL = 60 * time.Second)

type FlagsRepository struct{ db *pgxpool.Pool; rdb *redis.Client }

func NewFlagsRepository(db *pgxpool.Pool, rdb *redis.Client) *FlagsRepository {
	return &FlagsRepository{db: db, rdb: rdb}
}

func (r *FlagsRepository) GetAll(ctx context.Context) ([]domain.FeatureFlag, error) {
	rows, err := r.db.Query(ctx, `SELECT key,value,updated_at FROM feature_flags ORDER BY key`)
	if err != nil { return nil, err }
	defer rows.Close()
	var flags []domain.FeatureFlag
	for rows.Next() {
		var f domain.FeatureFlag
		if err := rows.Scan(&f.Key, &f.Value, &f.UpdatedAt); err != nil { return nil, err }
		flags = append(flags, f)
	}
	return flags, rows.Err()
}

func (r *FlagsRepository) Set(ctx context.Context, key, value string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO feature_flags(key,value,updated_at) VALUES($1,$2,NOW())
		ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW()`, key, value)
	if err != nil { return err }
	r.rdb.Del(ctx, flagsCacheKey)
	return nil
}

// GetPublicResponse builds FeatureFlagsResponse.
// CRITICAL: KillSwitch map must always be fully populated.
// Missing kill_switch_image_search defaults to false on the client.
func (r *FlagsRepository) GetPublicResponse(ctx context.Context) (*contractshttp.FeatureFlagsResponse, error) {
	if cached, err := r.rdb.Get(ctx, flagsCacheKey).Bytes(); err == nil {
		var resp contractshttp.FeatureFlagsResponse
		if json.Unmarshal(cached, &resp) == nil {
			return &resp, nil
		}
	}
	flags, err := r.GetAll(ctx)
	if err != nil { return nil, err }

	resp := &contractshttp.FeatureFlagsResponse{
		Flags:      make(map[string]bool),
		KillSwitch: make(map[string]bool),
		Params:     make(map[string]float64),
	}
	featureKeys := map[string]bool{"ai_enabled":true,"shop_enabled":true,"ws_enabled":true,"payments_enabled":true,"catalog_enabled":true}
	killSwitchKeys := map[string]bool{"kill_switch_ai":true,"kill_switch_ws":true,"kill_switch_payments":true,"kill_switch_catalog":true,"kill_switch_image_search":true}

	for _, f := range flags {
		switch {
		case featureKeys[f.Key]:
			resp.Flags[f.Key] = f.Value == "true"
		case killSwitchKeys[f.Key]:
			key := f.Key[len("kill_switch_"):]
			resp.KillSwitch[key] = f.Value == "true"
		case f.Key == domain.ParamRateSanityThresholdPct || f.Key == domain.ParamRateLimitBFFFreeRPM:
			v, _ := strconv.ParseFloat(f.Value, 64)
			resp.Params[f.Key] = v
		}
	}
	if _, ok := resp.KillSwitch["image_search"]; !ok {
		resp.KillSwitch["image_search"] = true // default blocked
	}
	b, _ := json.Marshal(resp)
	r.rdb.Set(ctx, flagsCacheKey, b, flagsCacheTTL)
	return resp, nil
}
