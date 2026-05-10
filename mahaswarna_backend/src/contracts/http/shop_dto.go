package http

type RegisterShopRequest struct {
	Name        string `json:"name"`
	CityID      string `json:"cityId"`
	Description string `json:"description,omitempty"`
	Phone       string `json:"phone,omitempty"`
}

type ShopResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	CityID      string `json:"cityId"`
	OwnerUserID string `json:"ownerUserId"`
	Description string `json:"description,omitempty"`
	Phone       string `json:"phone,omitempty"`
	BannerURL   string `json:"bannerUrl,omitempty"`
}
