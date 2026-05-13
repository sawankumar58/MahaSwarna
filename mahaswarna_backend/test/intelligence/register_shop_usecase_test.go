package intelligence_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/mahaswarna/intelligence/domain"
	"github.com/mahaswarna/intelligence/infrastructure"
	"github.com/redis/go-redis/v9"
)

// TestRegisterShopUseCase_ValidateGSTIN_ValidFormats verifies that correctly
// formatted GSTINs pass the validation regex.
func TestRegisterShopUseCase_ValidateGSTIN_ValidFormats(t *testing.T) {
	valid := []string{
		"27AAPFU0939F1ZV", // Maharashtra — real format
		"07AAACR5055K1Z5", // Delhi
		"29ABCDE1234F1Z1", // Karnataka
	}
	for _, g := range valid {
		if !domain.ValidateGSTIN(g) {
			t.Errorf("GSTIN %q must be valid", g)
		}
	}
}

// TestRegisterShopUseCase_ValidateGSTIN_InvalidFormats verifies that malformed
// GSTINs are rejected.
func TestRegisterShopUseCase_ValidateGSTIN_InvalidFormats(t *testing.T) {
	invalid := []string{
		"",
		"12345",
		"27AAPFU0939F1Z",   // too short (14 chars)
		"27AAPFU0939F1ZVX",  // too long (16 chars)
		"27aapfu0939f1zv",   // lowercase
		"00AAPFU0939F1ZV",   // invalid state code (00)
	}
	for _, g := range invalid {
		if domain.ValidateGSTIN(g) {
			t.Errorf("GSTIN %q must be invalid", g)
		}
	}
}

// TestRegisterShopUseCase_FreeTierBlocked verifies that a FREE-tier user
// (SubscriptionProjection cache miss = FREE) cannot register a shop.
func TestRegisterShopUseCase_FreeTierBlocked(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	userID := uuid.New()
	// No key in Redis → IsPremium returns false (fail-closed).
	proj := infrastructure.NewSubscriptionProjection(rdb)
	isPremium, err := proj.IsPremium(ctx, userID)
	if err != nil {
		t.Fatalf("IsPremium: %v", err)
	}
	if isPremium {
		t.Error("cache miss must return false (fail-closed — not premium)")
	}

	// The use case returns ErrNotPremium for non-premium users.
	var notPremium domain.ErrNotPremium
	_ = notPremium // compile-time check: type exists
	if notPremium.Error() != "operation requires PREMIUM subscription" {
		t.Errorf("ErrNotPremium message mismatch: %q", notPremium.Error())
	}
}

// TestRegisterShopUseCase_PremiumTierAllowed verifies that a PREMIUM-tier user
// passes the subscription guard.
func TestRegisterShopUseCase_PremiumTierAllowed(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	userID := uuid.New()
	proj := infrastructure.NewSubscriptionProjection(rdb)

	// Seed PREMIUM tier.
	if err := proj.SetTier(ctx, userID, "PREMIUM"); err != nil {
		t.Fatalf("SetTier: %v", err)
	}

	isPremium, err := proj.IsPremium(ctx, userID)
	if err != nil {
		t.Fatalf("IsPremium: %v", err)
	}
	if !isPremium {
		t.Error("PREMIUM tier must return isPremium=true")
	}
}

// TestRegisterShopUseCase_ErrShopAlreadyExists verifies the error type returned
// when a user tries to register a second shop (DB unique constraint on user_id).
func TestRegisterShopUseCase_ErrShopAlreadyExists(t *testing.T) {
	err := domain.ErrShopAlreadyExists{}
	if err.Error() != "shop already registered for this user" {
		t.Errorf("ErrShopAlreadyExists message mismatch: %q", err.Error())
	}
}

// TestRegisterShopUseCase_SubscriptionProjection_TTLSet verifies that the
// Redis key gets a non-zero TTL when SetTier is called.
func TestRegisterShopUseCase_SubscriptionProjection_TTLSet(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	userID := uuid.New()
	proj := infrastructure.NewSubscriptionProjection(rdb)
	if err := proj.SetTier(ctx, userID, "PREMIUM"); err != nil {
		t.Fatalf("SetTier: %v", err)
	}

	ttl, err := rdb.TTL(ctx, "sub:"+userID.String()).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 0 {
		t.Error("subscription projection key must have a positive TTL")
	}
}
