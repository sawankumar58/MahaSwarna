package domain

import "time"

type FeatureFlag struct {
	Key       string
	Value     string
	UpdatedAt time.Time
}

const (
	FlagAIEnabled = "ai_enabled"; FlagShopEnabled = "shop_enabled"
	FlagWSEnabled = "ws_enabled"; FlagPaymentsEnabled = "payments_enabled"; FlagCatalogEnabled = "catalog_enabled"
	KillSwitchAI = "kill_switch_ai"; KillSwitchWS = "kill_switch_ws"
	KillSwitchPayments = "kill_switch_payments"; KillSwitchCatalog = "kill_switch_catalog"
	KillSwitchImageSearch = "kill_switch_image_search"
	ParamRateSanityThresholdPct = "rate_sanity_threshold_pct"
	ParamRateLimitBFFFreeRPM    = "rate_limit_bff_free_rpm"
)
