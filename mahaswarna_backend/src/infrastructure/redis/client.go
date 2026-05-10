package redis

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewFailoverClient returns a Redis Sentinel failover client.
// Reads REDIS_SENTINEL_1/2/3 and REDIS_MASTER_NAME from environment.
// ARCHITECTURE INVARIANT: Never use redis.NewClient() — always NewFailoverClient()
// so automatic failover works correctly on primary failure (~10–30s recovery).
func NewFailoverClient() *redis.Client {
	sentinels := []string{
		mustEnv("REDIS_SENTINEL_1"),
		mustEnv("REDIS_SENTINEL_2"),
		mustEnv("REDIS_SENTINEL_3"),
	}

	masterName := os.Getenv("REDIS_MASTER_NAME")
	if masterName == "" {
		masterName = "mymaster"
	}

	password := os.Getenv("REDIS_PASSWORD")
	if password == "" {
		// REDIS_PASSWORD is intentionally optional in local/dev environments,
		// but must be set in staging and production. Log a warning so misconfigured
		// deployments are caught early rather than silently connecting unauthenticated.
		slog.Warn("REDIS_PASSWORD is not set — connecting without authentication; ensure this is intentional")
	}

	return redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:      masterName,
		SentinelAddrs:   sentinels,
		Password:        password,
		DB:              0,
		MaxRetries:      3,
		MinRetryBackoff: 100 * time.Millisecond,
		MaxRetryBackoff: 2 * time.Second,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolSize:        20,
		MinIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	})
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required env var %q is not set", key))
	}
	return v
}
