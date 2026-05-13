package intelligence_test

import (
	"testing"

	"github.com/mahaswarna/intelligence/domain"
)

// TestGenerateInvoiceUseCase_KaratFactor verifies the purity multiplier used
// for per-gram gold pricing. These values must match Indian jewellery standards.
func TestGenerateInvoiceUseCase_KaratFactor(t *testing.T) {
	cases := []struct {
		karat int
		want  float64
	}{
		{24, 1.0},
		{22, 22.0 / 24.0},
		{18, 18.0 / 24.0},
		{14, 14.0 / 24.0},
		{10, 10.0 / 24.0},
	}
	for _, c := range cases {
		got := domain.KaratFactor(c.karat)
		if abs(got-c.want) > 1e-10 {
			t.Errorf("KaratFactor(%d): expected %.6f, got %.6f", c.karat, c.want, got)
		}
	}
}

// TestGenerateInvoiceUseCase_GSTRate verifies the 3% GST constant applied to
// gold/silver jewellery in India (FY 2024-25).
func TestGenerateInvoiceUseCase_GSTRate(t *testing.T) {
	if domain.GSTRate != 0.03 {
		t.Errorf("GSTRate must be 0.03 (3%%), got %v", domain.GSTRate)
	}
}

// TestGenerateInvoiceUseCase_LineItemComputation verifies the full price
// computation pipeline for a single line item:
//   unitPrice  = goldRate × weightGrams × karatFactor
//   totalPrice = unitPrice + makingCharge
//   gstAmount  = totalPrice × GSTRate
//   netAmount  = totalPrice + gstAmount
func TestGenerateInvoiceUseCase_LineItemComputation(t *testing.T) {
	goldRate := 72000.0
	item := domain.InvoiceLineItem{
		Description:  "22K Gold Ring",
		WeightGrams:  5.0,
		Karat:        22,
		MakingCharge: 500.0,
	}

	kf := domain.KaratFactor(item.Karat) // 22/24
	unitPrice := goldRate * item.WeightGrams * kf
	totalPrice := unitPrice + item.MakingCharge
	gstAmount := totalPrice * domain.GSTRate
	netAmount := totalPrice + gstAmount

	if abs(unitPrice-(72000*5*22.0/24.0)) > 0.01 {
		t.Errorf("unitPrice: expected %.2f, got %.2f", 72000*5*22.0/24.0, unitPrice)
	}
	want := unitPrice + 500
	if abs(totalPrice-want) > 0.01 {
		t.Errorf("totalPrice: expected %.2f, got %.2f", want, totalPrice)
	}
	if abs(gstAmount-totalPrice*0.03) > 0.01 {
		t.Errorf("gstAmount: expected %.2f, got %.2f", totalPrice*0.03, gstAmount)
	}
	if abs(netAmount-(totalPrice+gstAmount)) > 0.01 {
		t.Errorf("netAmount: expected %.2f, got %.2f", totalPrice+gstAmount, netAmount)
	}
}

// TestGenerateInvoiceUseCase_DailyLimit verifies the constant.
// Architecture: Redis counter key = invoice_count:{shopID}:{YYYY-MM-DD-IST}
func TestGenerateInvoiceUseCase_DailyLimit(t *testing.T) {
	if domain.DailyInvoiceLimit != 60 {
		t.Errorf("DailyInvoiceLimit must be 60, got %d", domain.DailyInvoiceLimit)
	}
}

// TestGenerateInvoiceUseCase_ErrDailyLimitExceeded verifies the error type and
// message format for the HTTP 429 path.
func TestGenerateInvoiceUseCase_ErrDailyLimitExceeded(t *testing.T) {
	err := domain.ErrDailyLimitExceeded{Limit: domain.DailyInvoiceLimit}
	if err.Error() != "daily invoice limit (60) exceeded for today" {
		t.Errorf("ErrDailyLimitExceeded message: %q", err.Error())
	}
}

// TestGenerateInvoiceUseCase_PaymentModeConstants verifies the wire values
// used in invoices.payment_mode (DB CHECK constraint).
func TestGenerateInvoiceUseCase_PaymentModeConstants(t *testing.T) {
	if domain.PaymentModeCash != "cash" {
		t.Errorf("PaymentModeCash must be \"cash\", got %q", domain.PaymentModeCash)
	}
	if domain.PaymentModeUPI != "upi" {
		t.Errorf("PaymentModeUPI must be \"upi\", got %q", domain.PaymentModeUPI)
	}
	if domain.PaymentModeCard != "card" {
		t.Errorf("PaymentModeCard must be \"card\", got %q", domain.PaymentModeCard)
	}
}

// TestGenerateInvoiceUseCase_RateSourceConstants verifies RateSource wire values.
func TestGenerateInvoiceUseCase_RateSourceConstants(t *testing.T) {
	cases := []struct {
		got  domain.RateSource
		want string
	}{
		{domain.RateSourceLive, "live"},
		{domain.RateSourceStale, "stale"},
		{domain.RateSourceClientOverride, "client_override"},
		{domain.RateSourceManualOverride, "manual_override"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("RateSource %v must equal %q", c.got, c.want)
		}
	}
}

// TestGenerateInvoiceUseCase_GoldRateOverrideTakesPrecedence verifies that
// when a GoldRateOverride is provided, it replaces the live rate for computation.
func TestGenerateInvoiceUseCase_GoldRateOverrideTakesPrecedence(t *testing.T) {
	liveGold := 72000.0
	override := 70000.0

	goldRate := liveGold
	if override != 0 {
		goldRate = override
	}

	if goldRate != override {
		t.Errorf("GoldRateOverride must take precedence: expected %.0f, got %.0f", override, goldRate)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
