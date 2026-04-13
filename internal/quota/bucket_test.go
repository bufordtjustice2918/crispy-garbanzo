package quota

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckNoLimit(t *testing.T) {
	lim, _ := NewLimiter("/nonexistent")
	d := lim.Check("any-agent")
	if !d.Allowed {
		t.Fatal("no limit should allow")
	}
}

func TestCheckHardStop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quotas.json")
	os.WriteFile(path, []byte(`[{"agent_id":"a1","rps":1,"mode":"hard_stop"}]`), 0o644)

	lim, err := NewLimiter(path)
	if err != nil {
		t.Fatal(err)
	}

	// First request allowed (bucket starts full).
	d := lim.Check("a1")
	if !d.Allowed {
		t.Fatal("first request should be allowed")
	}

	// Immediate second request should be denied (1 RPS, no time passed).
	d = lim.Check("a1")
	if d.Allowed {
		t.Fatal("second immediate request should be denied with 1 RPS")
	}
	if d.Mode != "hard_stop" {
		t.Fatalf("want hard_stop mode, got %s", d.Mode)
	}
	if d.Reason == "" {
		t.Fatal("reason should be non-empty")
	}
}

func TestCheckAlertOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quotas.json")
	os.WriteFile(path, []byte(`[{"agent_id":"a1","rps":1,"mode":"alert_only"}]`), 0o644)

	lim, _ := NewLimiter(path)

	lim.Check("a1") // consume token
	d := lim.Check("a1")
	// alert_only: allowed even when over limit
	if !d.Allowed {
		t.Fatal("alert_only should still allow")
	}
	if d.Reason == "" {
		t.Fatal("should have a reason for alert")
	}
}

func TestCheckRecovery(t *testing.T) {
	lim, _ := NewLimiter("/nonexistent")
	lim.Set(Limit{AgentID: "a1", RPS: 100, Mode: "hard_stop"})

	// Consume all tokens.
	for range 100 {
		lim.Check("a1")
	}
	d := lim.Check("a1")
	if d.Allowed {
		t.Fatal("should be denied after exhausting 100 tokens")
	}

	// Wait for tokens to refill (100 RPS = 1 token per 10ms).
	time.Sleep(50 * time.Millisecond)
	d = lim.Check("a1")
	if !d.Allowed {
		t.Fatal("should recover after waiting")
	}
}

func TestSetRemoveSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quotas.json")

	lim, _ := NewLimiter(path)
	if len(lim.All()) != 0 {
		t.Fatal("expected empty")
	}

	lim.Set(Limit{AgentID: "a1", RPS: 10, Mode: "hard_stop"})
	if lim.LookupByID("a1") == nil {
		t.Fatal("set failed")
	}

	lim.Save()

	// Reload.
	lim2, _ := NewLimiter(path)
	if lim2.LookupByID("a1") == nil {
		t.Fatal("not persisted")
	}

	lim2.Remove("a1")
	if lim2.LookupByID("a1") != nil {
		t.Fatal("remove failed")
	}
}
