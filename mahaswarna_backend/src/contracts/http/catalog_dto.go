package http

type DesignSearchRequest struct {
	Query  string `json:"query"`
	CityID string `json:"cityId,omitempty"`
	Page   int    `json:"page,omitempty"`
	Size   int    `json:"size,omitempty"`
}

type DesignResponse struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	ImageURL    string   `json:"imageUrl"`
	ShopID      string   `json:"shopId"`
	CityID      string   `json:"cityId"`
	ViewCount   int64    `json:"viewCount"`
}

type DesignListResponse struct {
	Designs    []DesignResponse `json:"designs"`
	TotalCount int              `json:"totalCount"`
	Page       int              `json:"page"`
	TotalPages int              `json:"totalPages"`
}
