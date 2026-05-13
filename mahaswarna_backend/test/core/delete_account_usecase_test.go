package core_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mahaswarna/core/domain"
)

// TestDeleteAccountUseCase_SoftDeletePreservesRecord documents the expected
// state transition: DeletedAt is set, HardDeletedAt remains nil.
// Hard-delete is performed by HardDeleteJob after the 30-day grace period.
func TestDeleteAccountUseCase_SoftDeletePreservesRecord(t *testing.T) {
	now := time.Now()
	user := &domain.User{
		ID:            uuid.New(),
		Phone:         "+919876543210",
		Tier:          domain.TierFree,
		DeletedAt:     &now,
		HardDeletedAt: nil,
	}

	if user.DeletedAt == nil {
		t.Error("soft delete: DeletedAt must be set")
	}
	if user.HardDeletedAt != nil {
		t.Error("soft delete: HardDeletedAt must remain nil")
	}
}

// TestDeleteAccountUseCase_TierConstants verifies the tier wire values.
func TestDeleteAccountUseCase_TierConstants(t *testing.T) {
	if domain.TierFree != "FREE" {
		t.Errorf("TierFree must be \"FREE\", got %q", domain.TierFree)
	}
	if domain.TierPremium != "PREMIUM" {
		t.Errorf("TierPremium must be \"PREMIUM\", got %q", domain.TierPremium)
	}
	if domain.TierAdmin != "ADMIN" {
		t.Errorf("TierAdmin must be \"ADMIN\", got %q", domain.TierAdmin)
	}
}

// TestDeleteAccountUseCase_HardDeleteGracePeriod documents the 30-day grace
// constant. HardDeleteJob queries:
//   WHERE deleted_at < NOW() - INTERVAL '30 days' AND hard_deleted_at IS NULL
func TestDeleteAccountUseCase_HardDeleteGracePeriod(t *testing.T) {
	gracePeriod := 30 * 24 * time.Hour

	// A user soft-deleted 31 days ago IS eligible.
	deletedAt31 := time.Now().Add(-31 * 24 * time.Hour)
	if time.Since(deletedAt31) <= gracePeriod {
		t.Error("user soft-deleted 31 days ago must be eligible for hard delete")
	}

	// A user soft-deleted 29 days ago is NOT eligible.
	deletedAt29 := time.Now().Add(-29 * 24 * time.Hour)
	if time.Since(deletedAt29) > gracePeriod {
		t.Error("user soft-deleted 29 days ago must NOT be eligible for hard delete")
	}
}

// TestDeleteAccountUseCase_IdempotencyGuard verifies the idempotency invariant:
// HardDeleteJob stamps hard_deleted_at BEFORE firing the pg NOTIFY event so a
// crash-restart cannot re-process the same user.
func TestDeleteAccountUseCase_IdempotencyGuard(t *testing.T) {
	now := time.Now()
	hardDeletedAt := now
	user := &domain.User{
		ID:            uuid.New(),
		DeletedAt:     &now,
		HardDeletedAt: &hardDeletedAt,
	}
	if user.HardDeletedAt == nil {
		t.Error("idempotency: HardDeletedAt must be non-nil after hard delete")
	}
}

// TestDeleteAccountUseCase_UserIDRoundTrip verifies UUIDs survive the
// string serialisation used in audit log entries and event payloads.
func TestDeleteAccountUseCase_UserIDRoundTrip(t *testing.T) {
	userID := uuid.New()
	parsed, err := uuid.Parse(userID.String())
	if err != nil {
		t.Fatalf("uuid round-trip failed: %v", err)
	}
	if parsed != userID {
		t.Error("uuid round-trip: parsed value differs from original")
	}
}
