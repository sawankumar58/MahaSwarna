package infrastructure

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const viewCountKeyPrefix = "vc:"

// ViewCountCache manages a Redis buffer for design view counts.
// Increments are buffered here; flush_view_counts_job.go drains them to Postgres
// every 5 minutes via DesignRepository.BulkAddViewCounts.
//
// Key schema: vc:{designID} → integer (INCR)
type ViewCountCache struct {
	rdb *redis.Client
}

func NewViewCountCache(rdb *redis.Client) *ViewCountCache {
	return &ViewCountCache{rdb: rdb}
}

// Increment atomically increments the view count buffer for the given design.
func (v *ViewCountCache) Increment(ctx context.Context, designID uuid.UUID) error {
	key := viewCountKeyPrefix + designID.String()
	return v.rdb.Incr(ctx, key).Err()
}

// FlushAll reads and deletes all vc:* keys, returning a map of designID → count delta.
// Uses a Redis SCAN + pipeline pattern to avoid blocking the server.
// Uses GETDEL (Redis 6.2+). Compatible with the Redis Sentinel 3-node setup specified in ARCHITECTURE.md.
func (v *ViewCountCache) FlushAll(ctx context.Context) (map[uuid.UUID]int64, error) {
	var cursor uint64
	result := make(map[uuid.UUID]int64)

	for {
		var keys []string
		var err error
		keys, cursor, err = v.rdb.Scan(ctx, cursor, viewCountKeyPrefix+"*", 200).Result()
		if err != nil {
			return nil, fmt.Errorf("view count scan: %w", err)
		}

		if len(keys) > 0 {
			// Pipeline: GETDEL each key atomically.
			pipe := v.rdb.Pipeline()
			cmds := make([]*redis.StringCmd, len(keys))
			for i, k := range keys {
				cmds[i] = pipe.GetDel(ctx, k)
			}
			if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
				return nil, fmt.Errorf("view count pipeline exec: %w", err)
			}

			for i, cmd := range cmds {
				val, err := cmd.Int64()
				if err == redis.Nil {
					continue
				}
				if err != nil {
					return nil, fmt.Errorf("view count get %s: %w", keys[i], err)
				}
				// Extract UUID from key: vc:{uuid}
				rawID := strings.TrimPrefix(keys[i], viewCountKeyPrefix)
				id, err := uuid.Parse(rawID)
				if err != nil {
					// Malformed key — skip; do not abort the flush.
					continue
				}
				result[id] += val
			}
		}

		if cursor == 0 {
			break
		}
	}
	return result, nil
}
