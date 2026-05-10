package infrastructure

import (
	"fmt"
	"os"
)

// CDNURLBuilder converts an S3 object key to a public CDN URL.
// CDN_BASE_URL must end without a trailing slash (e.g. "https://cdn.mahaswarna.com").
type CDNURLBuilder struct {
	baseURL string
}

func NewCDNURLBuilder() (*CDNURLBuilder, error) {
	base := os.Getenv("CDN_BASE_URL")
	if base == "" {
		return nil, fmt.Errorf("CDN_BASE_URL not set")
	}
	return &CDNURLBuilder{baseURL: base}, nil
}

// Build returns the public CDN URL for a given S3 object key.
func (b *CDNURLBuilder) Build(objectKey string) string {
	return fmt.Sprintf("%s/%s", b.baseURL, objectKey)
}
