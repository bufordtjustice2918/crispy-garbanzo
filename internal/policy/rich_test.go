package policy

import "testing"

func TestEvaluateRichMethodFilter(t *testing.T) {
	eng := &Engine{}
	eng.rules = []Rule{
		{PolicyID: "get-only", AgentID: "*", Domains: []string{"api.example.com"}, Methods: []string{"GET"}, Action: "allow"},
		{PolicyID: "default-deny", AgentID: "*", Domains: []string{"*"}, Action: "deny"},
	}

	// GET allowed.
	d := eng.EvaluateRich(RequestContext{AgentID: "a1", Destination: "api.example.com", Method: "GET"})
	if d.Action != "allow" {
		t.Fatalf("GET should be allowed, got %s", d.Action)
	}

	// POST denied (method doesn't match get-only, falls to default-deny).
	d = eng.EvaluateRich(RequestContext{AgentID: "a1", Destination: "api.example.com", Method: "POST"})
	if d.Action != "deny" {
		t.Fatalf("POST should be denied, got %s", d.Action)
	}

	// Method matching is case-insensitive.
	d = eng.EvaluateRich(RequestContext{AgentID: "a1", Destination: "api.example.com", Method: "get"})
	if d.Action != "allow" {
		t.Fatalf("lowercase get should match, got %s", d.Action)
	}
}

func TestEvaluateRichPathPrefix(t *testing.T) {
	eng := &Engine{}
	eng.rules = []Rule{
		{PolicyID: "api-only", AgentID: "*", Domains: []string{"example.com"}, PathPrefixes: []string{"/api/", "/v1/"}, Action: "allow"},
		{PolicyID: "default-deny", AgentID: "*", Domains: []string{"*"}, Action: "deny"},
	}

	d := eng.EvaluateRich(RequestContext{AgentID: "a1", Destination: "example.com", Path: "/api/users"})
	if d.Action != "allow" {
		t.Fatalf("/api/users should be allowed, got %s", d.Action)
	}

	d = eng.EvaluateRich(RequestContext{AgentID: "a1", Destination: "example.com", Path: "/v1/data"})
	if d.Action != "allow" {
		t.Fatalf("/v1/data should be allowed, got %s", d.Action)
	}

	d = eng.EvaluateRich(RequestContext{AgentID: "a1", Destination: "example.com", Path: "/admin/secret"})
	if d.Action != "deny" {
		t.Fatalf("/admin/secret should be denied, got %s", d.Action)
	}
}

func TestEvaluateRichConditions(t *testing.T) {
	eng := &Engine{}
	eng.rules = []Rule{
		{PolicyID: "prod-only", AgentID: "*", Domains: []string{"*"}, Conditions: map[string]string{"environment": "prod"}, Action: "allow"},
		{PolicyID: "default-deny", AgentID: "*", Domains: []string{"*"}, Action: "deny"},
	}

	// Prod environment allowed.
	d := eng.EvaluateRich(RequestContext{AgentID: "a1", Destination: "example.com", Environment: "prod"})
	if d.Action != "allow" {
		t.Fatalf("prod should be allowed, got %s", d.Action)
	}

	// Dev environment denied.
	d = eng.EvaluateRich(RequestContext{AgentID: "a1", Destination: "example.com", Environment: "dev"})
	if d.Action != "deny" {
		t.Fatalf("dev should be denied, got %s", d.Action)
	}
}

func TestEvaluateRichTeamCondition(t *testing.T) {
	eng := &Engine{}
	eng.rules = []Rule{
		{PolicyID: "ops-team", AgentID: "*", Domains: []string{"*"}, Conditions: map[string]string{"team_id": "ops"}, Action: "allow"},
		{PolicyID: "default-deny", AgentID: "*", Domains: []string{"*"}, Action: "deny"},
	}

	d := eng.EvaluateRich(RequestContext{AgentID: "a1", Destination: "x", TeamID: "ops"})
	if d.Action != "allow" {
		t.Fatalf("ops team should be allowed")
	}

	d = eng.EvaluateRich(RequestContext{AgentID: "a1", Destination: "x", TeamID: "other"})
	if d.Action != "deny" {
		t.Fatalf("other team should be denied")
	}
}

func TestEvaluateRichCombined(t *testing.T) {
	eng := &Engine{}
	eng.rules = []Rule{
		{
			PolicyID:     "strict",
			AgentID:      "a1",
			Domains:      []string{"api.openai.com"},
			Methods:      []string{"GET", "POST"},
			PathPrefixes: []string{"/v1/"},
			Conditions:   map[string]string{"environment": "prod"},
			Action:       "allow",
		},
		{PolicyID: "default-deny", AgentID: "*", Domains: []string{"*"}, Action: "deny"},
	}

	// All criteria match.
	d := eng.EvaluateRich(RequestContext{
		AgentID: "a1", Destination: "api.openai.com",
		Method: "POST", Path: "/v1/chat/completions", Environment: "prod",
	})
	if d.Action != "allow" || d.PolicyID != "strict" {
		t.Fatalf("all criteria match, want allow/strict, got %s/%s", d.Action, d.PolicyID)
	}

	// Wrong method.
	d = eng.EvaluateRich(RequestContext{
		AgentID: "a1", Destination: "api.openai.com",
		Method: "DELETE", Path: "/v1/chat/completions", Environment: "prod",
	})
	if d.Action != "deny" {
		t.Fatalf("DELETE should be denied")
	}

	// Wrong path.
	d = eng.EvaluateRich(RequestContext{
		AgentID: "a1", Destination: "api.openai.com",
		Method: "GET", Path: "/admin/keys", Environment: "prod",
	})
	if d.Action != "deny" {
		t.Fatalf("wrong path should be denied")
	}

	// Wrong environment.
	d = eng.EvaluateRich(RequestContext{
		AgentID: "a1", Destination: "api.openai.com",
		Method: "GET", Path: "/v1/models", Environment: "staging",
	})
	if d.Action != "deny" {
		t.Fatalf("staging should be denied")
	}
}

func TestEvaluateRichBackwardCompat(t *testing.T) {
	// Rules without Methods/PathPrefixes/Conditions should work exactly as before.
	eng := &Engine{}
	eng.rules = []Rule{
		{PolicyID: "p1", AgentID: "a1", Domains: []string{"example.com"}, Action: "allow"},
	}

	d := eng.Evaluate("a1", "example.com")
	if d.Action != "allow" || d.PolicyID != "p1" {
		t.Fatalf("backward compat broken: got %s/%s", d.Action, d.PolicyID)
	}
}

func TestEvaluateRichEmptyMethodMatchesAll(t *testing.T) {
	eng := &Engine{}
	eng.rules = []Rule{
		{PolicyID: "p1", AgentID: "*", Domains: []string{"*"}, Action: "allow"},
	}

	// No Methods field = matches any method.
	d := eng.EvaluateRich(RequestContext{AgentID: "a1", Destination: "x", Method: "DELETE"})
	if d.Action != "allow" {
		t.Fatalf("empty Methods should match all methods")
	}
}
