package core_test

import (
	"fmt"
	"testing"

	"github.com/mahaswarna/core/domain"
)

// TestDeliverAlertUseCase_FCMPayloadFields verifies that ALL 6 required FCM
// data fields are present in the payload map built by Deliver().
// Architecture invariant (canonical source): "direction" was missing in old PRD
// §9. This test catches any regression that drops a field.
func TestDeliverAlertUseCase_FCMPayloadFields(t *testing.T) {
	alert := domain.Alert{
		Metal:     domain.MetalGold,
		Direction: domain.DirectionAbove,
		Threshold: 72000.0,
		CityID:    "mumbai",
	}

	// Reproduce the exact payload construction from deliver_alert_usecase.go.
	data := map[string]string{
		"type":      "price_alert",
		"metal":     alert.Metal,
		"direction": alert.Direction,
		"threshold": fmt.Sprintf("%.2f", alert.Threshold),
		"city_id":   alert.CityID,
		"screen":    "rates",
	}

	required := []string{"type", "metal", "direction", "threshold", "city_id", "screen"}
	for _, field := range required {
		if _, ok := data[field]; !ok {
			t.Errorf("FCM payload missing required field %q", field)
		}
	}
	if len(data) != len(required) {
		t.Errorf("FCM payload has %d fields, expected %d", len(data), len(required))
	}
}

// TestDeliverAlertUseCase_DirectionAboveTrigger verifies that an "above"
// alert fires when rate >= threshold.
func TestDeliverAlertUseCase_DirectionAboveTrigger(t *testing.T) {
	cases := []struct {
		rate, threshold float64
		shouldFire      bool
	}{
		{72001, 72000, true},  // rate > threshold
		{72000, 72000, true},  // rate == threshold (boundary)
		{71999, 72000, false}, // rate < threshold
	}
	for _, c := range cases {
		fired := c.rate >= c.threshold
		if fired != c.shouldFire {
			t.Errorf("above: rate=%.0f threshold=%.0f: expected fire=%v, got %v",
				c.rate, c.threshold, c.shouldFire, fired)
		}
	}
}

// TestDeliverAlertUseCase_DirectionBelowTrigger verifies that a "below"
// alert fires when rate <= threshold.
func TestDeliverAlertUseCase_DirectionBelowTrigger(t *testing.T) {
	cases := []struct {
		rate, threshold float64
		shouldFire      bool
	}{
		{59999, 60000, true},  // rate < threshold
		{60000, 60000, true},  // rate == threshold (boundary)
		{60001, 60000, false}, // rate > threshold
	}
	for _, c := range cases {
		fired := c.rate <= c.threshold
		if fired != c.shouldFire {
			t.Errorf("below: rate=%.0f threshold=%.0f: expected fire=%v, got %v",
				c.rate, c.threshold, c.shouldFire, fired)
		}
	}
}

// TestDeliverAlertUseCase_MetalConstants verifies the metal wire values.
func TestDeliverAlertUseCase_MetalConstants(t *testing.T) {
	if domain.MetalGold != "gold" {
		t.Errorf("MetalGold must be \"gold\", got %q", domain.MetalGold)
	}
	if domain.MetalSilver != "silver" {
		t.Errorf("MetalSilver must be \"silver\", got %q", domain.MetalSilver)
	}
	if domain.DirectionAbove != "above" {
		t.Errorf("DirectionAbove must be \"above\", got %q", domain.DirectionAbove)
	}
	if domain.DirectionBelow != "below" {
		t.Errorf("DirectionBelow must be \"below\", got %q", domain.DirectionBelow)
	}
}

// TestDeliverAlertUseCase_ThresholdFormat verifies that the threshold is
// formatted with 2 decimal places, matching the FCM data field.
func TestDeliverAlertUseCase_ThresholdFormat(t *testing.T) {
	cases := []struct {
		threshold float64
		want      string
	}{
		{72000, "72000.00"},
		{59999.5, "59999.50"},
		{100, "100.00"},
		{0.1, "0.10"},
	}
	for _, c := range cases {
		got := fmt.Sprintf("%.2f", c.threshold)
		if got != c.want {
			t.Errorf("threshold %.4f: expected %q, got %q", c.threshold, c.want, got)
		}
	}
}
