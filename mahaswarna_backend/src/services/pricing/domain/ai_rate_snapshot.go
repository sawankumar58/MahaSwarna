package domain

import "time"

// AIRateSnapshot represents a Gemini-generated gold/silver rate for a city at a point in time.
// This is the primary rate source. manual_override rows share the same structure.
type AIRateSnapshot struct {
	ID          string
	CityID      string
	Gold        float64
	Silver      float64
	Source      RateSource
	IsStale     bool
	GeneratedAt time.Time
	CreatedAt   time.Time
}
