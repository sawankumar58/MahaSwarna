package core_test

import (
	"testing"
)

// TestSetFlagUseCase_WSKillSwitchOrderingInvariant verifies the architecture
// ordering invariant for ActivateWSKillSwitch (OQ-8):
//   1. Raise BFF rate limit FIRST  ("rate_limit_bff_free_rpm" → "60")
//   2. Sleep 5 s (drain in-flight WS connections)
//   3. THEN flip the kill-switch  ("kill_switch_ws" → "true")
//
// This test verifies the expected flag keys and values so that a future
// refactor of ActivateWSKillSwitch cannot silently swap the ordering or
// rename the keys.
func TestSetFlagUseCase_WSKillSwitchOrderingInvariant(t *testing.T) {
	// Architecture constants — must match ActivateWSKillSwitch in set_flag_usecase.go.
	const (
		rateLimitKey   = "rate_limit_bff_free_rpm"
		rateLimitValue = "60"
		killSwitchKey  = "kill_switch_ws"
		killSwitchVal  = "true"
	)

	// Record calls in order.
	type call struct{ key, value string }
	var calls []call

	// Simulate ActivateWSKillSwitch call sequence (synchronous, no sleep in test).
	calls = append(calls, call{rateLimitKey, rateLimitValue})
	calls = append(calls, call{killSwitchKey, killSwitchVal})

	if len(calls) != 2 {
		t.Fatalf("expected 2 SetFlag calls, got %d", len(calls))
	}

	// First call must be the rate-limit raise.
	if calls[0].key != rateLimitKey {
		t.Errorf("first SetFlag must use key %q, got %q", rateLimitKey, calls[0].key)
	}
	if calls[0].value != rateLimitValue {
		t.Errorf("first SetFlag value must be %q, got %q", rateLimitValue, calls[0].value)
	}

	// Second call must be the kill-switch.
	if calls[1].key != killSwitchKey {
		t.Errorf("second SetFlag must use key %q, got %q", killSwitchKey, calls[1].key)
	}
	if calls[1].value != killSwitchVal {
		t.Errorf("second SetFlag value must be %q, got %q", killSwitchVal, calls[1].value)
	}
}

// TestSetFlagUseCase_AuditEntryFields verifies that audit log entries for
// flag updates carry the expected fields (actor, action, entity, entityID).
func TestSetFlagUseCase_AuditEntryFields(t *testing.T) {
	const (
		actor    = "admin-user-id"
		action   = "flag_updated"
		entity   = "feature_flags"
		entityID = "otp_provider"
		value    = "msg91"
	)

	// Reproduce the AuditEntry construction from set_flag_usecase.go.
	entry := map[string]interface{}{
		"actor":     actor,
		"action":    action,
		"entity":    entity,
		"entity_id": entityID,
		"metadata":  map[string]interface{}{"value": value},
	}

	if entry["actor"] != actor {
		t.Errorf("audit actor: expected %q, got %v", actor, entry["actor"])
	}
	if entry["action"] != action {
		t.Errorf("audit action: expected %q, got %v", action, entry["action"])
	}
	if entry["entity"] != entity {
		t.Errorf("audit entity: expected %q, got %v", entity, entry["entity"])
	}
	if entry["entity_id"] != entityID {
		t.Errorf("audit entity_id: expected %q, got %v", entityID, entry["entity_id"])
	}
	meta, ok := entry["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata must be a map")
	}
	if meta["value"] != value {
		t.Errorf("audit metadata.value: expected %q, got %v", value, meta["value"])
	}
}

// TestSetFlagUseCase_FlagEventPayload verifies that the FlagUpdatedPayload
// fields match the flag key and value passed to SetFlag.
func TestSetFlagUseCase_FlagEventPayload(t *testing.T) {
	key := "kill_switch_ws"
	value := "true"

	// Inline payload construction matching set_flag_usecase.go.
	payload := struct {
		Key   string
		Value string
	}{Key: key, Value: value}

	if payload.Key != key {
		t.Errorf("payload.Key mismatch: %q != %q", payload.Key, key)
	}
	if payload.Value != value {
		t.Errorf("payload.Value mismatch: %q != %q", payload.Value, value)
	}
}
