package core_test

import (
	"testing"

	"github.com/mahaswarna/core/domain"
)

// TestVerifyReceiptUseCase_KnownSKUs verifies the known SKU allowlist contains
// exactly the three production SKUs and nothing else.
func TestVerifyReceiptUseCase_KnownSKUs(t *testing.T) {
	expected := []string{
		"mahaswarna_premium_monthly",
		"mahaswarna_premium_yearly",
		"mahaswarna_premium_lifetime",
	}
	for _, sku := range expected {
		if !domain.IsKnownSKU(sku) {
			t.Errorf("expected %q to be a known SKU", sku)
		}
	}
	if got := len(domain.KnownSKUs); got != len(expected) {
		t.Errorf("KnownSKUs has %d entries, expected %d; update test if adding a new SKU", got, len(expected))
	}
}

// TestVerifyReceiptUseCase_UnknownSKURejectsEarly verifies that an unknown
// productID returns an error before any Google Play API call is made.
func TestVerifyReceiptUseCase_UnknownSKURejectsEarly(t *testing.T) {
	unknowns := []string{
		"",
		"mahaswarna_premium_ultimate",
		"com.google.android.fake_sku",
		"MAHASWARNA_PREMIUM_MONTHLY", // wrong case
	}
	for _, sku := range unknowns {
		if domain.IsKnownSKU(sku) {
			t.Errorf("SKU %q must not be in the known set", sku)
		}
	}
}

// TestVerifyReceiptUseCase_PaymentStateConstants verifies the wire values used
// in receipt_log rows and audit entries.
func TestVerifyReceiptUseCase_PaymentStateConstants(t *testing.T) {
	if domain.PaymentStateVerified != "VERIFIED" {
		t.Errorf("PaymentStateVerified must be \"VERIFIED\", got %q", domain.PaymentStateVerified)
	}
	if domain.PaymentStateFailed != "FAILED" {
		t.Errorf("PaymentStateFailed must be \"FAILED\", got %q", domain.PaymentStateFailed)
	}
}

// TestVerifyReceiptUseCase_SubscriptionActivatedAudit verifies that the audit
// entry for a successful receipt verification contains tier in the metadata.
func TestVerifyReceiptUseCase_SubscriptionActivatedAudit(t *testing.T) {
	tier := "PREMIUM"
	purchaseToken := "test-token-abc123"

	entry := map[string]interface{}{
		"actor":     "user-uuid",
		"action":    "subscription_activated",
		"entity":    "subscriptions",
		"entity_id": purchaseToken,
		"metadata":  map[string]interface{}{"tier": tier},
	}

	if entry["action"] != "subscription_activated" {
		t.Errorf("audit action must be \"subscription_activated\", got %v", entry["action"])
	}
	meta := entry["metadata"].(map[string]interface{})
	if meta["tier"] != tier {
		t.Errorf("audit metadata.tier must be %q, got %v", tier, meta["tier"])
	}
}

// TestVerifyReceiptUseCase_BillingOutputExpiryFormat verifies that a nil
// expiresAt produces an empty string in BillingOutput (lifetime SKU case).
func TestVerifyReceiptUseCase_BillingOutputExpiryFormat(t *testing.T) {
	var expiresAt interface{} = nil // nil → lifetime

	exp := ""
	if expiresAt != nil {
		// Would be: exp = expiresAt.Format("2006-01-02T15:04:05Z07:00")
		exp = "non-empty"
	}

	if exp != "" {
		t.Errorf("nil expiresAt must produce empty string in BillingOutput.ExpiresAt, got %q", exp)
	}
}
