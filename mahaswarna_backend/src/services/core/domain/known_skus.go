package domain

var KnownSKUs = map[string]bool{
	"mahaswarna_premium_monthly":  true,
	"mahaswarna_premium_yearly":   true,
	"mahaswarna_premium_lifetime": true,
}

func IsKnownSKU(productID string) bool { return KnownSKUs[productID] }
