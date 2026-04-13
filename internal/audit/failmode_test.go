package audit

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWriteToClosedLog verifies the gateway doesn't panic when the audit
// file is unavailable. The Write method should return an error, but the
// caller (gateway) logs and continues — it never blocks the request path.
func TestWriteToClosedLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	l, err := NewLog(path)
	if err != nil {
		t.Fatal(err)
	}

	// Close the underlying file to simulate a sink failure.
	l.Close()

	// Write should return an error, not panic.
	err = l.Write(Event{
		RequestID:   "r1",
		AgentID:     "a1",
		Decision:    "allow",
		PolicyID:    "p1",
		Destination: "example.com",
		Method:      "GET",
	})
	if err == nil {
		t.Fatal("expected error writing to closed log")
	}
}

// TestWriteToReadOnlyDir verifies graceful failure when the log directory
// is not writable.
func TestWriteToReadOnlyDir(t *testing.T) {
	dir := t.TempDir()
	roDir := filepath.Join(dir, "readonly")
	os.MkdirAll(roDir, 0o555)

	_, err := NewLog(filepath.Join(roDir, "sub", "audit.jsonl"))
	// Creating subdirs in a read-only dir should fail.
	if err == nil {
		t.Fatal("expected error creating log in read-only dir")
	}
}

// TestLogContinuesAfterPartialFailure verifies that a successful Write
// works, then a close+write fails, but subsequent operations on a new
// log succeed — simulating audit sink recovery.
func TestLogContinuesAfterPartialFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	l1, _ := NewLog(path)
	l1.Write(Event{RequestID: "r1", AgentID: "a1", Decision: "allow", PolicyID: "p1", Destination: "x", Method: "GET"})
	l1.Close()

	// Re-open — should work, simulating recovery.
	l2, err := NewLog(path)
	if err != nil {
		t.Fatalf("re-open failed: %v", err)
	}
	defer l2.Close()

	err = l2.Write(Event{RequestID: "r2", AgentID: "a2", Decision: "deny", PolicyID: "p2", Destination: "y", Method: "GET"})
	if err != nil {
		t.Fatalf("write after recovery failed: %v", err)
	}

	// Both events should be in the file.
	events, _ := Query(path, Filter{})
	if len(events) != 2 {
		t.Fatalf("expected 2 events after recovery, got %d", len(events))
	}
}
