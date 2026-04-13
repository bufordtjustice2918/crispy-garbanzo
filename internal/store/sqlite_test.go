package store

import (
	"path/filepath"
	"testing"

	"github.com/bufordtjustice2918/crispy-garbanzo/internal/identity"
	"github.com/bufordtjustice2918/crispy-garbanzo/internal/policy"
	"github.com/bufordtjustice2918/crispy-garbanzo/internal/quota"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestAgentCRUD(t *testing.T) {
	db := openTestDB(t)

	a := identity.Agent{AgentID: "a1", TeamID: "t1", ProjectID: "p1", Environment: "dev", APIKey: "k1", Status: "active"}
	if err := db.SaveAgent(a); err != nil {
		t.Fatal(err)
	}

	// Load into registry.
	reg, _ := identity.NewRegistry("/nonexistent")
	if err := db.LoadAgents(reg); err != nil {
		t.Fatal(err)
	}
	if got := reg.LookupByID("a1"); got == nil {
		t.Fatal("agent not loaded")
	}

	// Update.
	a.Status = "disabled"
	if err := db.SaveAgent(a); err != nil {
		t.Fatal(err)
	}

	reg2, _ := identity.NewRegistry("/nonexistent")
	db.LoadAgents(reg2)
	got := reg2.LookupByID("a1")
	if got == nil || got.Status != "disabled" {
		t.Fatalf("update failed, got %v", got)
	}

	// Delete.
	if err := db.DeleteAgent("a1"); err != nil {
		t.Fatal(err)
	}
	reg3, _ := identity.NewRegistry("/nonexistent")
	db.LoadAgents(reg3)
	if reg3.LookupByID("a1") != nil {
		t.Fatal("delete failed")
	}
}

func TestPolicyCRUD(t *testing.T) {
	db := openTestDB(t)

	r := policy.Rule{PolicyID: "p1", AgentID: "*", Domains: []string{"example.com", "*.test.com"}, Action: "allow"}
	if err := db.SavePolicy(r); err != nil {
		t.Fatal(err)
	}

	eng, _ := policy.NewEngine("/nonexistent")
	if err := db.LoadPolicies(eng); err != nil {
		t.Fatal(err)
	}
	rules := eng.Rules()
	if len(rules) != 1 {
		t.Fatalf("want 1 rule, got %d", len(rules))
	}
	if len(rules[0].Domains) != 2 {
		t.Fatalf("want 2 domains, got %d", len(rules[0].Domains))
	}

	// Delete.
	db.DeletePolicy("p1")
	eng2, _ := policy.NewEngine("/nonexistent")
	db.LoadPolicies(eng2)
	if len(eng2.Rules()) != 0 {
		t.Fatal("delete failed")
	}
}

func TestQuotaCRUD(t *testing.T) {
	db := openTestDB(t)

	q := quota.Limit{AgentID: "a1", RPS: 10, RPM: 100, Mode: "hard_stop"}
	if err := db.SaveQuota(q); err != nil {
		t.Fatal(err)
	}

	lim, _ := quota.NewLimiter("/nonexistent")
	if err := db.LoadQuotas(lim); err != nil {
		t.Fatal(err)
	}
	got := lim.LookupByID("a1")
	if got == nil || got.RPS != 10 {
		t.Fatalf("want RPS=10, got %v", got)
	}

	// Delete.
	db.DeleteQuota("a1")
	lim2, _ := quota.NewLimiter("/nonexistent")
	db.LoadQuotas(lim2)
	if lim2.LookupByID("a1") != nil {
		t.Fatal("delete failed")
	}
}
