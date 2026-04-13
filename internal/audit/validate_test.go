package audit

import "testing"

func TestValidateGood(t *testing.T) {
	e := Event{
		RequestID:   "r1",
		AgentID:     "a1",
		Decision:    "allow",
		PolicyID:    "p1",
		Destination: "example.com",
		Method:      "GET",
	}
	if err := Validate(e); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
}

func TestValidateMissingFields(t *testing.T) {
	cases := []struct {
		name  string
		event Event
	}{
		{"no request_id", Event{Decision: "allow", PolicyID: "p1", Destination: "x", Method: "GET"}},
		{"no decision", Event{RequestID: "r1", PolicyID: "p1", Destination: "x", Method: "GET"}},
		{"no policy_id", Event{RequestID: "r1", Decision: "allow", Destination: "x", Method: "GET"}},
		{"no destination", Event{RequestID: "r1", Decision: "allow", PolicyID: "p1", Method: "GET"}},
		{"no method", Event{RequestID: "r1", Decision: "allow", PolicyID: "p1", Destination: "x"}},
	}
	for _, tc := range cases {
		if err := Validate(tc.event); err == nil {
			t.Fatalf("%s: should fail validation", tc.name)
		}
	}
}

func TestValidateBadDecision(t *testing.T) {
	e := Event{
		RequestID:   "r1",
		Decision:    "maybe",
		PolicyID:    "p1",
		Destination: "x",
		Method:      "GET",
	}
	if err := Validate(e); err == nil {
		t.Fatal("invalid decision should fail")
	}
}
