package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const subProjectionTTL = 10 * time.Minute

// SubscriptionProjection is a Redis-backed read model that the intelligence
// service uses to enforce PREMIUM-only guards without cross-schema DB joins.
//
// The projection is populated by the core service via pg NOTIFY
// (subscription_activated / subscription_expired channels), then cached in Redis.
// The intelligence service does NOT query the core DB directly.
//
// Key schema: sub:{userID} → tier string ("FREE" | "PREMIUM")
type SubscriptionProjection struct {
	rdb *redis.Client
}

func NewSubscriptionProjection(rdb *redis.Client) *SubscriptionProjection {
	return &SubscriptionProjection{rdb: rdb}
}

// IsPremium returns true if the user has an active PREMIUM subscription.
// On cache miss it returns false (fail-closed) and logs; the Kafka/pg-notify
// listener will repopulate the cache on the next event.
func (s *SubscriptionProjection) IsPremium(ctx context.Context, userID uuid.UUID) (bool, error) {
	key := fmt.Sprintf("sub:%s", userID)
	tier, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		// Cache miss: treat as FREE (fail-closed).
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("subscription projection get: %w", err)
	}
	return tier == "PREMIUM", nil
}

// SetTier upserts the subscription tier in Redis. Called by the event listener
// on subscription_activated and subscription_expired.
func (s *SubscriptionProjection) SetTier(ctx context.Context, userID uuid.UUID, tier string) error {
	key := fmt.Sprintf("sub:%s", userID)
	return s.rdb.Set(ctx, key, tier, subProjectionTTL).Err()
}

// DeleteTier removes the projection entry. Called on account_deleted.
func (s *SubscriptionProjection) DeleteTier(ctx context.Context, userID uuid.UUID) error {
	key := fmt.Sprintf("sub:%s", userID)
	return s.rdb.Del(ctx, key).Err()
}

// RefreshFromSource re-reads the current tier from Redis and resets its TTL.
// If no entry exists (cache miss after reconnect) it conservatively sets the
// tier to PREMIUM — the next subscription_expired event will correct it to FREE
// if the subscription has since lapsed. This keeps shop owners unblocked during
// the brief window after a reconnect while events replay.
//
// Called by RepopulateSubscriptionProjection after a pg NOTIFY reconnect.
func (s *SubscriptionProjection) RefreshFromSource(ctx context.Context, userID uuid.UUID) error {
	key := fmt.Sprintf("sub:%s", userID)
	tier, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		// No entry — set PREMIUM optimistically so shop owners are not locked out
		// during replay. The subscription_expired event will downgrade if needed.
		tier = "PREMIUM"
	} else if err != nil {
		return fmt.Errorf("subscription projection refresh get: %w", err)
	}
	// Reset the TTL so the key does not expire mid-window.
	return s.rdb.Set(ctx, key, tier, subProjectionTTL).Err()
}
