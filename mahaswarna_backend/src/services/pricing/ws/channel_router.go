package ws

import (
	"encoding/json"
	"fmt"
)

// Channel identifies a WebSocket subscription channel.
type Channel string

const (
	ChannelRates  Channel = "rates"
	ChannelAlerts Channel = "alerts"
)

// InboundEnvelope is the JSON structure sent by the Android client over WebSocket.
//
//	{ "channel": "rates", "payload": { "cityId": "mumbai" } }
type InboundEnvelope struct {
	Channel Channel         `json:"channel"`
	Payload json.RawMessage `json:"payload"`
}

// RatesSubscribePayload is the payload for a rates channel subscription.
type RatesSubscribePayload struct {
	CityID string `json:"cityId"`
}

// OutboundRateMessage is pushed to clients when a city rate updates.
type OutboundRateMessage struct {
	Channel Channel `json:"channel"`
	CityID  string  `json:"cityId"`
	Gold    float64 `json:"gold"`
	Silver  float64 `json:"silver"`
	Stale   bool    `json:"stale"`
	Source  string  `json:"source"`
}

// OutboundAlertMessage is pushed to the originating user when core fires alert_delivered.
// Channel: "alerts". The Android client navigates to the rates screen on receipt.
type OutboundAlertMessage struct {
	Channel Channel `json:"channel"`
	AlertID string  `json:"alertId"`
	CityID  string  `json:"cityId"`
	Metal   string  `json:"metal"`
	Rate    float64 `json:"rate"`
}

// ParseEnvelope decodes raw WebSocket bytes into an InboundEnvelope.
func ParseEnvelope(data []byte) (*InboundEnvelope, error) {
	var env InboundEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("envelope parse: %w", err)
	}
	if env.Channel == "" {
		return nil, fmt.Errorf("envelope missing channel")
	}
	return &env, nil
}
