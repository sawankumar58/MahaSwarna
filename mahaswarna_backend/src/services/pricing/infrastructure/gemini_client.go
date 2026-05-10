package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiRateResult holds the AI-generated gold and silver rates for one city.
type GeminiRateResult struct {
	CityID string
	Gold   float64
	Silver float64
}

// GeminiClient wraps the Google Generative AI SDK to fetch gold/silver rates.
// ARCHITECTURE INVARIANT: Gemini is the sole rate source. The API key is
// server-only and MUST NEVER be forwarded to the Android client.
type GeminiClient struct {
	model *genai.GenerativeModel
}

func NewGeminiClient(ctx context.Context) (*GeminiClient, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("gemini client init: %w", err)
	}

	model := client.GenerativeModel("gemini-1.5-flash")
	// Keep responses deterministic and concise — we only need JSON, not prose.
	model.SetTemperature(0.1)
	model.SetMaxOutputTokens(256)

	return &GeminiClient{model: model}, nil
}

// geminiRateJSON is the expected JSON response shape from Gemini.
type geminiRateJSON struct {
	Gold   float64 `json:"gold"`   // INR per gram, 24K
	Silver float64 `json:"silver"` // INR per gram
}

// GenerateRate calls Gemini to get the current gold and silver spot prices for cityID.
// The caller is responsible for enforcing the 2-second per-city deadline via ctx.
//
// Prompt strategy: single-shot JSON only. No conversational framing.
// Gemini is instructed to return ONLY a JSON object with no markdown fence or preamble,
// because any extra text will cause JSON unmarshal to fail and trigger the stale path.
func (g *GeminiClient) GenerateRate(ctx context.Context, cityID string) (*GeminiRateResult, error) {
	prompt := fmt.Sprintf(
		`You are a gold market data API. Return ONLY a valid JSON object with no markdown, `+
			`no explanation, no code fences. Format: {"gold": <number>, "silver": <number>}. `+
			`Values must be current Indian retail spot prices in INR per gram for %s. `+
			`Gold: 24K hallmark rate. Silver: 999 fine. Round to 2 decimal places.`,
		cityID,
	)

	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate %s: %w", cityID, err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini empty response for %s", cityID)
	}

	raw, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return nil, fmt.Errorf("gemini unexpected part type for %s", cityID)
	}

	// Strip any accidental markdown fences Gemini occasionally adds despite the prompt.
	cleaned := strings.TrimSpace(string(raw))
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var rates geminiRateJSON
	if err := json.Unmarshal([]byte(cleaned), &rates); err != nil {
		slog.Warn("gemini parse error", "city", cityID, "raw", cleaned, "err", err)
		return nil, fmt.Errorf("gemini parse %s: %w", cityID, err)
	}

	if rates.Gold <= 0 || rates.Silver <= 0 {
		return nil, fmt.Errorf("gemini zero/negative rate for %s: gold=%.2f silver=%.2f",
			cityID, rates.Gold, rates.Silver)
	}

	return &GeminiRateResult{
		CityID: cityID,
		Gold:   rates.Gold,
		Silver: rates.Silver,
	}, nil
}
