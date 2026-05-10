package domain

import (
	"time"

	"github.com/google/uuid"
)

// PaymentMode constrains invoice.payment_mode (matches DB CHECK constraint).
type PaymentMode string

const (
	PaymentModeCash PaymentMode = "cash"
	PaymentModeUPI  PaymentMode = "upi"
	PaymentModeCard PaymentMode = "card"
)

// RateSource indicates where the metal rate used in the invoice originated.
type RateSource string

const (
	RateSourceLive           RateSource = "live"
	RateSourceStale          RateSource = "stale"
	RateSourceClientOverride RateSource = "client_override"
	RateSourceManualOverride RateSource = "manual_override"
)

// InvoiceLineItem is the JSON structure stored in invoices.items (JSONB).
type InvoiceLineItem struct {
	Description  string  `json:"description"`
	WeightGrams  float64 `json:"weightGrams"`
	Karat        int     `json:"karat"`
	MakingCharge float64 `json:"makingCharge,omitempty"`
	UnitPrice    float64 `json:"unitPrice"`    // gold_rate * weight * karat_factor
	TotalPrice   float64 `json:"totalPrice"`   // unitPrice + makingCharge
	GSTAmount    float64 `json:"gstAmount"`    // 3% GST on totalPrice
	NetAmount    float64 `json:"netAmount"`    // totalPrice + gstAmount
}

// Invoice represents a generated invoice record (ADR-001: PDF bytes NOT stored server-side).
type Invoice struct {
	ID                  uuid.UUID        `db:"id"`
	ShopID              uuid.UUID        `db:"shop_id"`
	UserID              uuid.UUID        `db:"user_id"`
	CustomerName        string           `db:"customer_name"`
	CustomerPhone       *string          `db:"customer_phone"`
	Items               []InvoiceLineItem // marshalled/unmarshalled from JSONB
	PaymentMode         PaymentMode      `db:"payment_mode"`
	Notes               *string          `db:"notes"`
	GoldRateOverride    *float64         `db:"gold_rate_override"`
	SilverRateOverride  *float64         `db:"silver_rate_override"`
	RateSource          RateSource       `db:"rate_source"`
	PDFSizeBytes        *int             `db:"pdf_size_bytes"` // audit only; no PDF stored
	GeneratedAt         time.Time        `db:"generated_at"`
}

// KaratFactor converts a karat value to the gold purity ratio used for per-gram pricing.
func KaratFactor(karat int) float64 {
	return float64(karat) / 24.0
}

// GSTRate is the GST percentage applied to gold/silver jewellery in India (as of FY 2024-25).
const GSTRate = 0.03

// DailyInvoiceLimit is the maximum number of invoices a shop can generate per IST day.
// Enforced via Redis counter (key: invoice_count:{shopID}:{YYYY-MM-DD-IST}).
const DailyInvoiceLimit = 60
