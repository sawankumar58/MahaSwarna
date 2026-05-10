package domain

import "time"

// Metal identifies the precious metal.
type Metal string

const (
	MetalGold   Metal = "gold"
	MetalSilver Metal = "silver"
)

// RateSource identifies the origin of a rate value.
type RateSource string

const (
	SourceGemini         RateSource = "gemini"
	SourceManualOverride RateSource = "manual_override"
	SourceStale          RateSource = "stale"
)

// GoldRate is the canonical domain object for a city's gold and silver prices.
type GoldRate struct {
	CityID      string
	Gold        float64
	Silver      float64
	Source      RateSource
	IsStale     bool
	GeneratedAt time.Time
}
