package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func seedPolicy(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "policy.json")
	data := `[
		{"policy_id":"p1","agent_id":"a1","domains":["example.com"],"action":"allow"},
		{"policy_id":"p2","agent_id":"*","domains":["*.evil.com"],"action":"deny"},
		{"policy_id":"p3","agent_id":"*","domains":["*"],"action":"allow"}
	]`
	os.WriteFile(path, []byte(data), 0o644)
	return path
}

func TestEvaluateFirstMatchWins(t *testing.T) {
	dir := t.TempDir()
	eng, _ := NewEngine(seedPolicy(t, dir))

	// a1 + example.com → allow (p1)
	d := eng.Evaluate("a1", "example.com")
	if d.Action != "allow" || d.PolicyID != "p1" {
		t.Fatalf("want allow/p1, got %s/%s", d.Action, d.PolicyID)
	}

	// a1 + sub.evil.com → deny (p2, wildcard agent)
	d = eng.Evaluate("a1", "sub.evil.com")
	if d.Action != "deny" || d.PolicyID != "p2" {
		t.Fatalf("want deny/p2, got %s/%s", d.Action, d.PolicyID)
	}

	// a1 + anything.else → allow (p3, catch-all)
	d = eng.Evaluate("a1", "anything.else")
	if d.Action != "allow" || d.PolicyID != "p3" {
		t.Fatalf("want allow/p3, got %s/%s", d.Action, d.PolicyID)
	}

	// unknown agent + example.com → doesn't match p1 (agent_id=a1), matches p3
	d = eng.Evaluate("other", "example.com")
	if d.Action != "allow" || d.PolicyID != "p3" {
		t.Fatalf("want allow/p3 for other agent, got %s/%s", d.Action, d.PolicyID)
	}
}

func TestEvaluateDefaultDeny(t *testing.T) {
	eng, _ := NewEngine("/nonexistent") // no rules = default deny
	d := eng.Evaluate("a1", "example.com")
	if d.Action != "deny" || d.PolicyID != "default-deny" {
		t.Fatalf("want deny/default-deny, got %s/%s", d.Action, d.PolicyID)
	}
}

func TestStripPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	os.WriteFile(path, []byte(`[{"policy_id":"p1","agent_id":"*","domains":["example.com"],"action":"allow"}]`), 0o644)
	eng, _ := NewEngine(path)

	d := eng.Evaluate("a1", "example.com:443")
	if d.Action != "allow" {
		t.Fatalf("port should be stripped, got %s", d.Action)
	}
}

func TestWildcardDomain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	os.WriteFile(path, []byte(`[{"policy_id":"p1","agent_id":"*","domains":["*.example.com"],"action":"allow"}]`), 0o644)
	eng, _ := NewEngine(path)

	// sub.example.com matches *.example.com
	if d := eng.Evaluate("a1", "sub.example.com"); d.Action != "allow" {
		t.Fatalf("want allow for sub.example.com")
	}
	// example.com itself also matches *.example.com
	if d := eng.Evaluate("a1", "example.com"); d.Action != "allow" {
		t.Fatalf("want allow for example.com")
	}
	// other.com does not match
	if d := eng.Evaluate("a1", "other.com"); d.Action != "deny" {
		t.Fatalf("want deny for other.com")
	}
}

func TestAddRemoveSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")

	eng, _ := NewEngine(path) // missing = empty
	if len(eng.Rules()) != 0 {
		t.Fatal("expected empty")
	}

	eng.Add(Rule{PolicyID: "r1", AgentID: "*", Domains: []string{"x.com"}, Action: "allow"})
	if len(eng.Rules()) != 1 {
		t.Fatal("expected 1 rule")
	}

	// Replace by policy_id.
	eng.Add(Rule{PolicyID: "r1", AgentID: "*", Domains: []string{"y.com"}, Action: "deny"})
	if len(eng.Rules()) != 1 {
		t.Fatal("expected still 1 rule after replace")
	}
	if eng.Rules()[0].Domains[0] != "y.com" {
		t.Fatal("replace didn't update")
	}

	eng.Save()

	// Reload.
	eng2, _ := NewEngine(path)
	if len(eng2.Rules()) != 1 {
		t.Fatal("reload lost rules")
	}

	eng2.Remove("r1")
	if len(eng2.Rules()) != 0 {
		t.Fatal("remove failed")
	}
}
