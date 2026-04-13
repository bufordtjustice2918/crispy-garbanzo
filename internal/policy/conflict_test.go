package policy

import "testing"

func TestDetectConflictsNone(t *testing.T) {
	rules := []Rule{
		{PolicyID: "p1", AgentID: "a1", Domains: []string{"good.com"}, Action: "allow"},
		{PolicyID: "p2", AgentID: "a2", Domains: []string{"bad.com"}, Action: "deny"},
	}
	conflicts := DetectConflicts(rules)
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %d", len(conflicts))
	}
}

func TestDetectConflictsSameAgentSameDomain(t *testing.T) {
	rules := []Rule{
		{PolicyID: "p1", AgentID: "a1", Domains: []string{"example.com"}, Action: "allow"},
		{PolicyID: "p2", AgentID: "a1", Domains: []string{"example.com"}, Action: "deny"},
	}
	conflicts := DetectConflicts(rules)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Severity != "shadowed" {
		t.Fatalf("expected shadowed, got %s", conflicts[0].Severity)
	}
}

func TestDetectConflictsWildcardAgent(t *testing.T) {
	rules := []Rule{
		{PolicyID: "p1", AgentID: "*", Domains: []string{"example.com"}, Action: "allow"},
		{PolicyID: "p2", AgentID: "a1", Domains: []string{"example.com"}, Action: "deny"},
	}
	conflicts := DetectConflicts(rules)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict (wildcard agent overlaps), got %d", len(conflicts))
	}
}

func TestDetectConflictsWildcardDomain(t *testing.T) {
	rules := []Rule{
		{PolicyID: "p1", AgentID: "a1", Domains: []string{"*"}, Action: "allow"},
		{PolicyID: "p2", AgentID: "a1", Domains: []string{"evil.com"}, Action: "deny"},
	}
	conflicts := DetectConflicts(rules)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict (* overlaps evil.com), got %d", len(conflicts))
	}
}

func TestDetectConflictsSubdomain(t *testing.T) {
	rules := []Rule{
		{PolicyID: "p1", AgentID: "*", Domains: []string{"*.example.com"}, Action: "allow"},
		{PolicyID: "p2", AgentID: "*", Domains: []string{"example.com"}, Action: "deny"},
	}
	conflicts := DetectConflicts(rules)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict (*.example.com overlaps example.com), got %d", len(conflicts))
	}
}

func TestDetectConflictsSameAction(t *testing.T) {
	rules := []Rule{
		{PolicyID: "p1", AgentID: "*", Domains: []string{"example.com"}, Action: "allow"},
		{PolicyID: "p2", AgentID: "*", Domains: []string{"example.com"}, Action: "allow"},
	}
	conflicts := DetectConflicts(rules)
	if len(conflicts) != 0 {
		t.Fatalf("same action should not conflict, got %d", len(conflicts))
	}
}

func TestDeterministicOrdering(t *testing.T) {
	// First-match-wins: order matters. Same rules, different order, different result.
	rules1 := []Rule{
		{PolicyID: "allow-all", AgentID: "*", Domains: []string{"*"}, Action: "allow"},
		{PolicyID: "deny-evil", AgentID: "*", Domains: []string{"evil.com"}, Action: "deny"},
	}
	rules2 := []Rule{
		{PolicyID: "deny-evil", AgentID: "*", Domains: []string{"evil.com"}, Action: "deny"},
		{PolicyID: "allow-all", AgentID: "*", Domains: []string{"*"}, Action: "allow"},
	}

	eng1 := &Engine{}
	eng1.mu.Lock()
	eng1.rules = rules1
	eng1.mu.Unlock()

	eng2 := &Engine{}
	eng2.mu.Lock()
	eng2.rules = rules2
	eng2.mu.Unlock()

	// Order 1: allow-all first → evil.com is allowed
	d1 := eng1.Evaluate("a1", "evil.com")
	if d1.Action != "allow" {
		t.Fatalf("rules1: expected allow (catch-all first), got %s", d1.Action)
	}

	// Order 2: deny-evil first → evil.com is denied
	d2 := eng2.Evaluate("a1", "evil.com")
	if d2.Action != "deny" {
		t.Fatalf("rules2: expected deny (deny first), got %s", d2.Action)
	}

	// Proves ordering is deterministic and first-match-wins.
	if d1.Action == d2.Action {
		t.Fatal("different ordering should produce different results")
	}
}
