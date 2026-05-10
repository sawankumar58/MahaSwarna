package http

type VerifyReceiptRequest struct {
	PurchaseToken string `json:"purchaseToken"`
	ProductID     string `json:"productId"`
	PackageName   string `json:"packageName"`
}

type BillingResponse struct {
	Tier      string `json:"tier"`
	ExpiresAt string `json:"expiresAt,omitempty"` // RFC3339 or empty for lifetime
}
