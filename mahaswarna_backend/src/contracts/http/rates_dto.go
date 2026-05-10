package http

import "time"

type RateResponse struct {
	CityID    string    `json:"cityId"`
	Gold      float64   `json:"gold"`
	Silver    float64   `json:"silver"`
	Stale     bool      `json:"stale"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type RateHistoryResponse struct {
	CityID  string         `json:"cityId"`
	History []RateDataPoint `json:"history"`
}

type RateDataPoint struct {
	Gold      float64   `json:"gold"`
	Silver    float64   `json:"silver"`
	Timestamp time.Time `json:"timestamp"`
}

type AIRateResponse struct {
	CityID  string  `json:"cityId"`
	Gold    float64 `json:"gold"`
	Silver  float64 `json:"silver"`
	Stale   bool    `json:"stale"`
	Source  string  `json:"source"` // "live" | "stale" | "manual_override" | "client_override"
}
