package http

import "time"

type CreateAlertRequest struct {
	CityID    string  `json:"cityId"`
	Metal     string  `json:"metal"`      // "gold" | "silver"
	Threshold float64 `json:"threshold"`
	Direction string  `json:"direction"`  // "above" | "below"
}

type AlertResponse struct {
	ID          string     `json:"id"`
	CityID      string     `json:"cityId"`
	Metal       string     `json:"metal"`
	Threshold   float64    `json:"threshold"`
	Direction   string     `json:"direction"`
	CreatedAt   time.Time  `json:"createdAt"`
	DeliveredAt *time.Time `json:"deliveredAt,omitempty"`
}

type AlertListResponse struct {
	Alerts []AlertResponse `json:"alerts"`
}
