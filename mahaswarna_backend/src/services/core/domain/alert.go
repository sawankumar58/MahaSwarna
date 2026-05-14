package domain

import "time"
import "github.com/google/uuid"

type Alert struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	CityID      string
	Metal       string
	Threshold   float64
	Direction   string
	CreatedAt   time.Time
	DeliveredAt *time.Time
}

const (MetalGold = "gold"; MetalSilver = "silver"; DirectionAbove = "above"; DirectionBelow = "below")

// FCMAlertPayload defines all 6 required fields for FCM data-message alerts.
type FCMAlertPayload struct {
	Type      string `json:"type"`
	Metal     string `json:"metal"`
	Direction string `json:"direction"`
	Threshold string `json:"threshold"`
	CityID    string `json:"city_id"`
	Screen    string `json:"screen"`
}
