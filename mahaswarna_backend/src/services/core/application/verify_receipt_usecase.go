package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mahaswarna/core/domain"
	"github.com/mahaswarna/core/events"
	"github.com/mahaswarna/core/infrastructure"
	"github.com/mahaswarna/shared"
	ce "github.com/mahaswarna/contracts/events"
)

type VerifyReceiptUseCase struct {
	receiptLog *infrastructure.ReceiptLogRepository
	subRepo    *infrastructure.SubscriptionRepository
	userRepo   *infrastructure.UserRepository
	play       *infrastructure.GooglePlayClient
	audit      *infrastructure.AuditLogRepository
	notifier   *events.Notifier
}

func NewVerifyReceiptUseCase(rl *infrastructure.ReceiptLogRepository, sr *infrastructure.SubscriptionRepository,
	ur *infrastructure.UserRepository, play *infrastructure.GooglePlayClient,
	audit *infrastructure.AuditLogRepository, notifier *events.Notifier) *VerifyReceiptUseCase {
	return &VerifyReceiptUseCase{receiptLog: rl, subRepo: sr, userRepo: ur, play: play, audit: audit, notifier: notifier}
}

type ReceiptInput struct{ UserID uuid.UUID; PurchaseToken, ProductID, PackageName string }
type BillingOutput struct{ Tier, ExpiresAt string }

func (uc *VerifyReceiptUseCase) Execute(ctx context.Context, in ReceiptInput) (*BillingOutput, error) {
	if !domain.IsKnownSKU(in.ProductID) { return nil, fmt.Errorf("unknown sku: %s", in.ProductID) }
	tier, expiresAt, raw, err := uc.play.VerifySubscriptionPurchase(ctx, in.ProductID, in.PurchaseToken)
	if err != nil {
		uc.receiptLog.Insert(ctx, in.UserID, in.PurchaseToken, in.ProductID, in.PackageName, domain.PaymentStateFailed, map[string]any{"error": err.Error()})
		return nil, fmt.Errorf("play verify: %w", err)
	}
	uc.receiptLog.Insert(ctx, in.UserID, in.PurchaseToken, in.ProductID, in.PackageName, domain.PaymentStateVerified, raw)
	if err := uc.subRepo.Upsert(ctx, domain.Subscription{
		UserID: in.UserID, Tier: tier, PurchaseToken: in.PurchaseToken,
		ProductID: in.ProductID, PackageName: in.PackageName, ExpiresAt: expiresAt,
	}); err != nil { return nil, fmt.Errorf("upsert sub: %w", err) }
	uc.userRepo.UpdateTier(ctx, in.UserID, tier)
	uc.notifier.Notify(ctx, ce.ChannelSubscriptionActivated, ce.SubscriptionChangedPayload{UserID: in.UserID.String(), Tier: tier, Status: "ACTIVE"})
	uc.audit.Append(ctx, shared.AuditEntry{Actor: in.UserID.String(), Action: "subscription_activated", Entity: "subscriptions", EntityID: in.PurchaseToken, Metadata: map[string]any{"tier": tier}})
	exp := ""
	if expiresAt != nil { exp = expiresAt.Format("2006-01-02T15:04:05Z07:00") }
	return &BillingOutput{Tier: tier, ExpiresAt: exp}, nil
}
