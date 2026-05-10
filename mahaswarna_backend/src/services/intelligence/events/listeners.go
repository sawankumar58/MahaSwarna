package events

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	ce "github.com/mahaswarna/contracts/events"
	"github.com/mahaswarna/infrastructure/pgnotify"
	"github.com/mahaswarna/intelligence/infrastructure"
)

// RegisterListeners wires pg NOTIFY handlers for events the intelligence
// service cares about:
//
//   - subscription_activated → update SubscriptionProjection to PREMIUM
//   - subscription_expired   → update SubscriptionProjection to FREE
//   - account_deleted        → GDPR erasure: delete shops, invoices, projection
func RegisterListeners(
	listener *pgnotify.Listener,
	subProj *infrastructure.SubscriptionProjection,
	shops *infrastructure.ShopRepository,
	invoices *infrastructure.InvoiceRepository,
	s3 *infrastructure.S3Client,
) {
	listener.On(ce.ChannelSubscriptionActivated, func(ctx context.Context, _, payload string) {
		p, err := pgnotify.Unmarshal[ce.SubscriptionChangedPayload](payload)
		if err != nil {
			slog.Error("subscription_activated unmarshal", "err", err)
			return
		}
		userID, err := uuid.Parse(p.UserID)
		if err != nil {
			slog.Error("subscription_activated parse userID", "err", err)
			return
		}
		if err := subProj.SetTier(ctx, userID, p.Tier); err != nil {
			slog.Error("subscription_activated set tier", "userID", p.UserID, "err", err)
		}
	})

	listener.On(ce.ChannelSubscriptionExpired, func(ctx context.Context, _, payload string) {
		p, err := pgnotify.Unmarshal[ce.SubscriptionExpiredPayload](payload)
		if err != nil {
			slog.Error("subscription_expired unmarshal", "err", err)
			return
		}
		userID, err := uuid.Parse(p.UserID)
		if err != nil {
			slog.Error("subscription_expired parse userID", "err", err)
			return
		}
		if err := subProj.SetTier(ctx, userID, "FREE"); err != nil {
			slog.Error("subscription_expired set tier", "userID", p.UserID, "err", err)
		}
	})

	listener.On(ce.ChannelAccountDeleted, func(ctx context.Context, _, payload string) {
		p, err := pgnotify.Unmarshal[ce.AccountDeletedPayload](payload)
		if err != nil {
			slog.Error("account_deleted unmarshal", "err", err)
			return
		}
		userID, err := uuid.Parse(p.UserID)
		if err != nil {
			slog.Error("account_deleted parse userID", "err", err)
			return
		}
		handleAccountDeleted(ctx, userID, subProj, shops, invoices, s3)
	})
}

// handleAccountDeleted performs GDPR erasure for the intelligence service:
// delete shop invoices, delete S3 banner, delete shop, remove subscription projection.
func handleAccountDeleted(
	ctx context.Context,
	userID uuid.UUID,
	subProj *infrastructure.SubscriptionProjection,
	shops *infrastructure.ShopRepository,
	invoices *infrastructure.InvoiceRepository,
	s3 *infrastructure.S3Client,
) {
	shop, err := shops.GetByUserID(ctx, userID)
	if err != nil {
		slog.Error("account_deleted get shop", "userID", userID, "err", err)
		// Continue anyway — best-effort erasure.
	}

	if shop != nil {
		// Delete invoices first (FK constraint: invoices.shop_id → shops.id).
		if err := invoices.DeleteByShopID(ctx, shop.ID); err != nil {
			slog.Error("account_deleted delete invoices", "shopID", shop.ID, "err", err)
		}
		// Delete S3 banner.
		if shop.BannerObjectKey != nil && *shop.BannerObjectKey != "" {
			if err := s3.DeleteObject(ctx, *shop.BannerObjectKey); err != nil {
				slog.Warn("account_deleted delete banner", "key", *shop.BannerObjectKey, "err", err)
			}
		}
		// Delete shop.
		if err := shops.DeleteByUserID(ctx, userID); err != nil {
			slog.Error("account_deleted delete shop", "userID", userID, "err", err)
		}
	}

	// Remove subscription projection from Redis.
	if err := subProj.DeleteTier(ctx, userID); err != nil {
		slog.Error("account_deleted delete sub projection", "userID", userID, "err", err)
	}

	slog.Info("account_deleted erasure complete", "userID", userID)
}
