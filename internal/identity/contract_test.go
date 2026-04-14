package identity

import (
	"encoding/json"
	"testing"
)

// TestAgentJSONContract verifies the Agent struct serializes to the expected schema.
func TestAgentJSONContract(t *testing.T) {
	a := Agent{
		AgentID:     "a1",
		TeamID:      "t1",
		ProjectID:   "p1",
		Environment: "prod",
		APIKey:      "k1",
		Status:      "active",
	}
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}

	// Parse back as generic map to verify field names.
	var m map[string]any
	json.Unmarshal(data, &m)

	requiredFields := []string{"agent_id", "team_id", "project_id", "environment", "api_key", "status"}
	for _, f := range requiredFields {
		if _, ok := m[f]; !ok {
			t.Fatalf("missing required field %q in JSON", f)
		}
	}

	// No unexpected fields.
	if len(m) != len(requiredFields) {
		t.Fatalf("expected %d fields, got %d — unknown fields in JSON", len(requiredFields), len(m))
	}
}

// TestAgentJSONBackwardCompat verifies old JSON format still deserializes.
func TestAgentJSONBackwardCompat(t *testing.T) {
	// Minimal JSON (only required fields).
	old := `{"agent_id":"a1","api_key":"k1","status":"active"}`
	var a Agent
	if err := json.Unmarshal([]byte(old), &a); err != nil {
		t.Fatalf("old format should still parse: %v", err)
	}
	if a.AgentID != "a1" {
		t.Fatal("agent_id not parsed")
	}
}
