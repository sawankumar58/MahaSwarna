package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/mahaswarna/infrastructure/pgnotify"
	ce "github.com/mahaswarna/contracts/events"
	"github.com/redis/go-redis/v9"
)

type Listeners struct {
	db       *pgxpool.Pool
	rdb      *redis.Client
	subRepo  *infrastructure.SubscriptionRepository
	userRepo *infrastructure.UserRepository
	flagRepo *infrastructure.FlagsRepository
	ready    chan struct{}
}

func NewListeners(db *pgxpool.Pool, rdb *redis.Client, subRepo *infrastructure.SubscriptionRepository,
	userRepo *infrastructure.UserRepository, flagRepo *infrastructure.FlagsRepository) *Listeners {
	return &Listeners{db: db, rdb: rdb, subRepo: subRepo, userRepo: userRepo, flagRepo: flagRepo, ready: make(chan struct{})}
}

func (l *Listeners) Ready() <-chan struct{} { return l.ready }

func (l *Listeners) Start(ctx context.Context) {
	channels := []string{ce.ChannelSubscriptionActivated, ce.ChannelSubscriptionExpired, ce.ChannelUserCreated}
	listener := pgnotify.NewListener(l.db, channels, l.rebuild)
	if err := l.rebuild(ctx); err != nil {
		slog.Error("startup projection rebuild failed", "err", err)
	}
	close(l.ready)
	go listener.Listen(ctx)
}

func (l *Listeners) rebuild(ctx context.Context) error {
	// Catch-up: provision free subscriptions for users created during restart window.
	users, err := l.userRepo.GetRecentUsersWithoutSubscription(ctx, 5*time.Minute)
	if err != nil { return err }
	for _, u := range users {
		l.subRepo.InsertFree(ctx, u.ID)
	}
	l.flagRepo.GetPublicResponse(ctx) // refresh flags cache
	return nil
}
