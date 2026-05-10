package domain

import (
	"time"

	"github.com/google/uuid"
)

// MetalType constrains design_catalog.metal_type (matches DB CHECK constraint).
type MetalType string

const (
	MetalTypeGold   MetalType = "gold"
	MetalTypeSilver MetalType = "silver"
	MetalTypeBoth   MetalType = "both"
)

// Design represents a row from design_catalog.
// view_count is maintained via Redis INCR + periodic flush; never increment in the DB per-request.
type Design struct {
	ID          uuid.UUID `db:"id"`
	Title       string    `db:"title"`
	Description string    `db:"description"`
	Category    string    `db:"category"`
	Style       string    `db:"style"`
	Region      *string   `db:"region"`    // nil = all regions
	MetalType   MetalType `db:"metal_type"`
	ImageURL    string    `db:"image_url"`
	Tags        []string  `db:"tags"`
	ViewCount   int64     `db:"view_count"`
	ShopID      *uuid.UUID // populated from JOIN when available; not a DB column
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// SearchParams carries validated parameters from a catalog search request.
type SearchParams struct {
	Query     string
	Region    string // empty = no filter
	MetalType MetalType
	Page      int
	PageSize  int
}

// SearchResult is the paginated response from a catalog search.
type SearchResult struct {
	Designs    []Design
	TotalCount int
	Page       int
	TotalPages int
}
