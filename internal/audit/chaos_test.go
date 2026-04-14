package audit

import (
	"os"
	"path/filepath"
	"testing"
)

// TestQueryCorruptedLog verifies Query handles corrupted JSONL gracefully.
func TestQueryCorruptedLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	// Mix of valid and invalid lines.
	content := `{"request_id":"r1","agent_id":"a1","decision":"allow","policy_id":"p1","destination":"x","http_method":"GET"}
THIS IS NOT JSON
{"request_id":"r2","agent_id":"a2","decision":"deny","policy_id":"p2","destination":"y","http_method":"GET"}
{broken json
{"request_id":"r3","agent_id":"a1","decision":"allow","policy_id":"p1","destination":"z","http_method":"GET"}
`
	os.WriteFile(path, []byte(content), 0o644)

	events, err := Query(path, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	// Should skip corrupted lines and return the 3 valid ones.
	if len(events) != 3 {
		t.Fatalf("expected 3 valid events from corrupted log, got %d", len(events))
	}
}

// TestQueryEmptyFile verifies empty file doesn't error.
func TestQueryEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	os.WriteFile(path, []byte(""), 0o644)

	events, err := Query(path, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatal("empty file should return no events")
	}
}

// TestQueryEmptyLines verifies blank lines in JSONL are handled.
func TestQueryEmptyLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	content := `

{"request_id":"r1","agent_id":"a1","decision":"allow","policy_id":"p1","destination":"x","http_method":"GET"}

`
	os.WriteFile(path, []byte(content), 0o644)

	events, err := Query(path, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

// TestWriteAfterDiskFull simulates disk full by writing to /dev/full.
func TestWriteAfterDiskFull(t *testing.T) {
	// /dev/full returns ENOSPC on write — simulates disk full.
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("/dev/full not available")
	}

	l := &Log{path: "/dev/full"}
	f, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		t.Skip("can't open /dev/full")
	}
	l.f = f

	err = l.Write(Event{RequestID: "r1", Decision: "allow", PolicyID: "p1", Destination: "x", Method: "GET"})
	if err == nil {
		t.Fatal("write to full disk should return error")
	}
}
