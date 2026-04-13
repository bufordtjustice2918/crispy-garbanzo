package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func seedFile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "agents.json")
	data := `[
		{"agent_id":"a1","team_id":"t1","project_id":"p1","environment":"dev","api_key":"key1","status":"active"},
		{"agent_id":"a2","team_id":"t1","project_id":"p1","environment":"dev","api_key":"key2","status":"disabled"}
	]`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewRegistry(t *testing.T) {
	dir := t.TempDir()
	path := seedFile(t, dir)

	reg, err := NewRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(reg.All()); got != 2 {
		t.Fatalf("want 2 agents, got %d", got)
	}
}

func TestLookupByKey(t *testing.T) {
	dir := t.TempDir()
	path := seedFile(t, dir)

	reg, _ := NewRegistry(path)

	// Active agent found.
	a := reg.LookupByKey("key1")
	if a == nil || a.AgentID != "a1" {
		t.Fatalf("want a1, got %v", a)
	}

	// Disabled agent returns nil.
	if got := reg.LookupByKey("key2"); got != nil {
		t.Fatalf("disabled agent should return nil, got %v", got)
	}

	// Unknown key.
	if got := reg.LookupByKey("nope"); got != nil {
		t.Fatalf("unknown key should return nil, got %v", got)
	}
}

func TestLookupByID(t *testing.T) {
	dir := t.TempDir()
	path := seedFile(t, dir)
	reg, _ := NewRegistry(path)

	a := reg.LookupByID("a1")
	if a == nil || a.APIKey != "key1" {
		t.Fatalf("want a1, got %v", a)
	}
	if got := reg.LookupByID("missing"); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}

func TestAddRemoveSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agents.json")

	reg, _ := NewRegistry(path) // missing file = empty
	if len(reg.All()) != 0 {
		t.Fatal("expected empty registry")
	}

	reg.Add(Agent{AgentID: "new1", APIKey: "k1", Status: "active"})
	if got := reg.LookupByKey("k1"); got == nil {
		t.Fatal("added agent not found by key")
	}

	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}

	// Reload from disk.
	reg2, err := NewRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if reg2.LookupByID("new1") == nil {
		t.Fatal("saved agent not found after reload")
	}

	// Remove.
	if !reg2.Remove("new1") {
		t.Fatal("remove returned false")
	}
	if reg2.LookupByID("new1") != nil {
		t.Fatal("removed agent still found")
	}
	if reg2.Remove("new1") {
		t.Fatal("double remove should return false")
	}
}

func TestMissingFile(t *testing.T) {
	reg, err := NewRegistry("/nonexistent/path/agents.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.All()) != 0 {
		t.Fatal("missing file should give empty registry")
	}
}
