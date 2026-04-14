package policy

import (
	"encoding/json"
	"testing"
)

// TestRuleJSONContract verifies Rule serializes with all expected fields.
func TestRuleJSONContract(t *testing.T) {
	r := Rule{
		PolicyID:     "p1",
		AgentID:      "a1",
		Domains:      []string{"example.com"},
		Methods:      []string{"GET"},
		PathPrefixes: []string{"/api/"},
		Conditions:   map[string]string{"environment": "prod"},
		Action:       "allow",
	}
	data, _ := json.Marshal(r)

	var m map[string]any
	json.Unmarshal(data, &m)

	required := []string{"policy_id", "agent_id", "domains", "action"}
	for _, f := range required {
		if _, ok := m[f]; !ok {
			t.Fatalf("missing required field %q", f)
		}
	}

	// Optional fields present when set.
	optional := []string{"methods", "path_prefixes", "conditions"}
	for _, f := range optional {
		if _, ok := m[f]; !ok {
			t.Fatalf("expected optional field %q when set", f)
		}
	}
}

// TestRuleJSONBackwardCompat verifies old flat rules still parse.
func TestRuleJSONBackwardCompat(t *testing.T) {
	// Old format: no methods, path_prefixes, conditions.
	old := `{"policy_id":"p1","agent_id":"*","domains":["example.com"],"action":"allow"}`
	var r Rule
	if err := json.Unmarshal([]byte(old), &r); err != nil {
		t.Fatalf("old format should parse: %v", err)
	}
	if r.PolicyID != "p1" {
		t.Fatal("policy_id not parsed")
	}
	if len(r.Methods) != 0 {
		t.Fatal("methods should be empty for old format")
	}
	if len(r.PathPrefixes) != 0 {
		t.Fatal("path_prefixes should be empty for old format")
	}
	if len(r.Conditions) != 0 {
		t.Fatal("conditions should be empty for old format")
	}
}

// TestRuleOmitsEmptyOptionalFields verifies empty optional fields are omitted from JSON.
func TestRuleOmitsEmptyOptionalFields(t *testing.T) {
	r := Rule{PolicyID: "p1", AgentID: "*", Domains: []string{"*"}, Action: "deny"}
	data, _ := json.Marshal(r)

	var m map[string]any
	json.Unmarshal(data, &m)

	for _, f := range []string{"methods", "path_prefixes", "conditions"} {
		if _, ok := m[f]; ok {
			t.Fatalf("empty optional field %q should be omitted (omitempty)", f)
		}
	}
}
