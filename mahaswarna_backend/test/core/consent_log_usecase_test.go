package core_test

import (
	"testing"

	"github.com/mahaswarna/core/domain"
	"github.com/mahaswarna/shared"
)

// TestLogConsentUseCase_ValidConsentTypes verifies that only "privacy_policy"
// and "tos" are in the ValidConsentTypes allowlist.
// Architecture invariant: "ai_disclaimer" must NEVER appear in the allowlist.
func TestLogConsentUseCase_ValidConsentTypes(t *testing.T) {
	valid := domain.ValidConsentTypes

	if !valid[domain.ConsentTypePrivacyPolicy] {
		t.Errorf("expected %q to be a valid consent type", domain.ConsentTypePrivacyPolicy)
	}
	if !valid[domain.ConsentTypeTOS] {
		t.Errorf("expected %q to be a valid consent type", domain.ConsentTypeTOS)
	}
	if valid["ai_disclaimer"] {
		t.Error("ai_disclaimer must NOT be a valid consent type (architecture invariant)")
	}
}

// TestLogConsentUseCase_InvalidConsentTypeError verifies that the use case
// returns ErrInvalidConsentType for unknown consent types.
// We validate the sentinel and the allowlist gate in isolation.
func TestLogConsentUseCase_InvalidConsentTypeError(t *testing.T) {
	invalidTypes := []string{
		"ai_disclaimer",
		"marketing",
		"",
		"PRIVACY_POLICY", // wrong case
		"tos_v2",
	}
	for _, ct := range invalidTypes {
		if domain.ValidConsentTypes[ct] {
			t.Errorf("consent type %q must not be in the valid set", ct)
		}
	}
	// The use case returns this sentinel for any type not in the allowlist.
	if shared.ErrInvalidConsentType == nil {
		t.Fatal("shared.ErrInvalidConsentType must be non-nil")
	}
}

// TestLogConsentUseCase_AllowlistExhaustive verifies that exactly 2 consent
// types are valid, preventing silent addition of new types.
func TestLogConsentUseCase_AllowlistExhaustive(t *testing.T) {
	const expectedCount = 2
	if got := len(domain.ValidConsentTypes); got != expectedCount {
		t.Errorf("ValidConsentTypes has %d entries, expected %d; update this test if adding a new type", got, expectedCount)
	}
}

// TestConsentLog_ConsentTypeConstants verifies wire values match the domain
// constants used by the backend.
func TestConsentLog_ConsentTypeConstants(t *testing.T) {
	if domain.ConsentTypePrivacyPolicy != "privacy_policy" {
		t.Errorf("ConsentTypePrivacyPolicy wire value must be \"privacy_policy\", got %q", domain.ConsentTypePrivacyPolicy)
	}
	if domain.ConsentTypeTOS != "tos" {
		t.Errorf("ConsentTypeTOS wire value must be \"tos\", got %q", domain.ConsentTypeTOS)
	}
}
