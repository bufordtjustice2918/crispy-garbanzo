package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSampleLog(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "audit.jsonl")
	lines := `{"timestamp":"2026-01-01T00:00:01Z","request_id":"r1","agent_id":"a1","decision":"allow","policy_id":"p1","destination":"example.com","http_method":"GET"}
{"timestamp":"2026-01-01T00:00:02Z","request_id":"r2","agent_id":"a2","decision":"deny","policy_id":"p2","destination":"evil.com","http_method":"CONNECT"}
{"timestamp":"2026-01-01T00:00:03Z","request_id":"r3","agent_id":"a1","decision":"deny","policy_id":"p2","destination":"bad.com","http_method":"GET"}
{"timestamp":"2026-01-01T00:00:04Z","request_id":"r4","agent_id":"a1","decision":"allow","policy_id":"p1","destination":"good.com","http_method":"GET"}
`
	os.WriteFile(path, []byte(lines), 0o644)
	return path
}

func TestQueryAll(t *testing.T) {
	dir := t.TempDir()
	path := writeSampleLog(t, dir)

	events, err := Query(path, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("want 4 events, got %d", len(events))
	}
}

func TestQueryByAgent(t *testing.T) {
	dir := t.TempDir()
	path := writeSampleLog(t, dir)

	events, _ := Query(path, Filter{AgentID: "a1"})
	if len(events) != 3 {
		t.Fatalf("want 3 events for a1, got %d", len(events))
	}
	for _, e := range events {
		if e.AgentID != "a1" {
			t.Fatalf("wrong agent: %s", e.AgentID)
		}
	}
}

func TestQueryByDecision(t *testing.T) {
	dir := t.TempDir()
	path := writeSampleLog(t, dir)

	events, _ := Query(path, Filter{Decision: "deny"})
	if len(events) != 2 {
		t.Fatalf("want 2 deny events, got %d", len(events))
	}
}

func TestQueryLimit(t *testing.T) {
	dir := t.TempDir()
	path := writeSampleLog(t, dir)

	events, _ := Query(path, Filter{Limit: 2})
	if len(events) != 2 {
		t.Fatalf("want 2 events with limit, got %d", len(events))
	}
	// Should return the last 2.
	if events[0].RequestID != "r3" || events[1].RequestID != "r4" {
		t.Fatalf("limit should return last 2, got %s and %s", events[0].RequestID, events[1].RequestID)
	}
}

func TestQuerySince(t *testing.T) {
	dir := t.TempDir()
	path := writeSampleLog(t, dir)

	events, _ := Query(path, Filter{Since: "2026-01-01T00:00:03Z"})
	if len(events) != 2 {
		t.Fatalf("want 2 events since 00:00:03, got %d", len(events))
	}
}

func TestQueryCombined(t *testing.T) {
	dir := t.TempDir()
	path := writeSampleLog(t, dir)

	events, _ := Query(path, Filter{AgentID: "a1", Decision: "allow"})
	if len(events) != 2 {
		t.Fatalf("want 2 allow events for a1, got %d", len(events))
	}
}

func TestQueryMissingFile(t *testing.T) {
	events, err := Query("/nonexistent/audit.jsonl", Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if events != nil {
		t.Fatal("missing file should return nil")
	}
}
