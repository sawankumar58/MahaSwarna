package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PricingClient fetches live metal rates from the pricing service.
// Internal endpoint: http://pricing:4002/internal/rates/{cityID}
type PricingClient struct {
	baseURL    string
	httpClient *http.Client
}

// LiveRates holds the current gold and silver spot prices per gram.
type LiveRates struct {
	GoldPerGram   float64 `json:"goldPerGram"`
	SilverPerGram float64 `json:"silverPerGram"`
}

func NewPricingClient(baseURL string) *PricingClient {
	return &PricingClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// FetchRates calls the pricing service and returns live rates for the given city.
// cityID may be empty; the pricing service defaults to the national rate in that case.
func (c *PricingClient) FetchRates(ctx context.Context, cityID string) (*LiveRates, error) {
	path := "/internal/rates"
	if cityID != "" {
		path = fmt.Sprintf("/internal/rates/%s", cityID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("pricing request build: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pricing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pricing service returned %d", resp.StatusCode)
	}

	var rates LiveRates
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		return nil, fmt.Errorf("pricing response decode: %w", err)
	}
	return &rates, nil
}
