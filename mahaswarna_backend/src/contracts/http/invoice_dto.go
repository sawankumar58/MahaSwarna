package http

// InvoiceResponse — ADR-001: JSON wrapper with base64-encoded PDF bytes.
// PdfBytes is []byte; encoding/json base64-encodes it automatically.
// On Android, kotlinx.serialization decodes it to ByteArray automatically.
type InvoiceResponse struct {
	InvoiceID   string `json:"invoice_id"`
	PdfBytes    []byte `json:"pdf_bytes"`
	GeneratedAt string `json:"generated_at"` // RFC3339 with +05:30
	RateSource  string `json:"rate_source"`  // "live" | "stale" | "client_override" | "manual_override"
}

type GenerateInvoiceRequest struct {
	ShopID          string  `json:"shopId"`
	CustomerName    string  `json:"customerName"`
	Items           []InvoiceItem `json:"items"`
	GoldRateOverride float64 `json:"goldRateOverride,omitempty"`
}

type InvoiceItem struct {
	Description string  `json:"description"`
	WeightGrams float64 `json:"weightGrams"`
	Karat       int     `json:"karat"`
	MakingCharge float64 `json:"makingCharge,omitempty"`
}
