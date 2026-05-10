package events

const ChannelShopRegistered = "shop_registered"

type ShopRegisteredPayload struct {
	ShopID string `json:"shop_id"`
	UserID string `json:"user_id"`
	CityID string `json:"city_id"`
}
