package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bufordtjustice2918/crispy-garbanzo/internal/policy"
)

// TestReplayDeterministic verifies that replaying audit events through the
// policy engine produces the same decisions. This guards against policy
// evaluation regressions.
func TestReplayDeterministic(t *testing.T) {
	dir := t.TempDir()

	// Set up a policy engine.
	policyPath := filepath.Join(dir, "policy.json")
	os.WriteFile(policyPath, []byte(`[
		{"policy_id":"allow-local","agent_id":"a1","domains":["localhost","127.0.0.1"],"action":"allow"},
		{"policy_id":"deny-evil","agent_id":"*","domains":["*.evil.com"],"action":"deny"},
		{"policy_id":"default-deny","agent_id":"*","domains":["*"],"action":"deny"}
	]`), 0o644)
	eng, err := policy.NewEngine(policyPath)
	if err != nil {
		t.Fatal(err)
	}

	// Write audit log with known decisions.
	auditPath := filepath.Join(dir, "audit.jsonl")
	os.WriteFile(auditPath, []byte(`{"request_id":"r1","agent_id":"a1","destination":"localhost","http_method":"GET","decision":"allow","policy_id":"allow-local"}
{"request_id":"r2","agent_id":"a1","destination":"sub.evil.com","http_method":"CONNECT","decision":"deny","policy_id":"deny-evil"}
{"request_id":"r3","agent_id":"a2","destination":"anything.com","http_method":"GET","decision":"deny","policy_id":"default-deny"}
`), 0o644)

	// Replay: re-evaluate each event and compare decisions.
	events, err := Query(auditPath, Filter{})
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range events {
		dec := eng.Evaluate(e.AgentID, e.Destination)
		if dec.Action != e.Decision {
			t.Errorf("replay %s: want %s, got %s (policy %s vs %s)",
				e.RequestID, e.Decision, dec.Action, e.PolicyID, dec.PolicyID)
		}
		if dec.PolicyID != e.PolicyID {
			t.Errorf("replay %s: policy mismatch: want %s, got %s",
				e.RequestID, e.PolicyID, dec.PolicyID)
		}
	}
}

// TestReplayDetectsDrift verifies that a policy change causes replay mismatches.
func TestReplayDetectsDrift(t *testing.T) {
	dir := t.TempDir()

	// Original policy: allow localhost.
	policyPath := filepath.Join(dir, "policy.json")
	os.WriteFile(policyPath, []byte(`[
		{"policy_id":"default-deny","agent_id":"*","domains":["*"],"action":"deny"}
	]`), 0o644)
	eng, _ := policy.NewEngine(policyPath)

	// Audit log recorded an "allow" decision under a different policy.
	auditPath := filepath.Join(dir, "audit.jsonl")
	os.WriteFile(auditPath, []byte(`{"request_id":"r1","agent_id":"a1","destination":"localhost","http_method":"GET","decision":"allow","policy_id":"allow-local"}
`), 0o644)

	events, _ := Query(auditPath, Filter{})
	dec := eng.Evaluate(events[0].AgentID, events[0].Destination)
	if dec.Action == events[0].Decision {
		t.Fatal("replay should detect drift — policy changed but decision didn't")
	}
}
