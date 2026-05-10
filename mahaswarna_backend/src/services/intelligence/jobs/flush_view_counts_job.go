package jobs

import (
	"context"
	"log/slog"

	"github.com/mahaswarna/intelligence/infrastructure"
)

// FlushViewCountsJob drains the Redis view count buffer into Postgres.
// Scheduled every 5 minutes by the cron runner in main.go.
//
// Pattern: Redis GETDEL (atomic get-and-delete) → bulk UPDATE in Postgres.
// This ensures no view counts are lost even if the flush partially fails:
// GETDEL only removes counts that were successfully read by this job.
type FlushViewCountsJob struct {
	cache   *infrastructure.ViewCountCache
	designs *infrastructure.DesignRepository
}

func NewFlushViewCountsJob(
	cache *infrastructure.ViewCountCache,
	designs *infrastructure.DesignRepository,
) *FlushViewCountsJob {
	return &FlushViewCountsJob{cache: cache, designs: designs}
}

// Run is the cron entrypoint. It is idempotent: an empty flush is a no-op.
func (j *FlushViewCountsJob) Run() {
	ctx := context.Background()

	counts, err := j.cache.FlushAll(ctx)
	if err != nil {
		slog.Error("view count flush: read from Redis", "err", err)
		return
	}
	if len(counts) == 0 {
		return
	}

	if err := j.designs.BulkAddViewCounts(ctx, counts); err != nil {
		slog.Error("view count flush: write to Postgres", "err", err, "designs", len(counts))
		return
	}
	slog.Info("view count flush complete", "designs_updated", len(counts))
}
