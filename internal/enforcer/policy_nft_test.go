package enforcer

import (
	"strings"
	"testing"

	"github.com/bufordtjustice2918/crispy-garbanzo/internal/policy"
)

func TestRenderPolicyNftBasic(t *testing.T) {
	rules := []policy.Rule{
		{PolicyID: "p1", AgentID: "*", Domains: []string{"127.0.0.1"}, Action: "allow"},
		{PolicyID: "p2", AgentID: "*", Domains: []string{"10.0.0.1"}, Action: "deny"},
	}

	out := RenderPolicyNft(rules, "clawgress", "egress_policy")
	if !strings.Contains(out, "table inet clawgress") {
		t.Fatal("missing table declaration")
	}
	if !strings.Contains(out, "set policy_allow") {
		t.Fatal("missing allow set")
	}
	if !strings.Contains(out, "127.0.0.1") {
		t.Fatal("missing allow IP")
	}
	if !strings.Contains(out, "set policy_deny") {
		t.Fatal("missing deny set")
	}
	if !strings.Contains(out, "10.0.0.1") {
		t.Fatal("missing deny IP")
	}
	if !strings.Contains(out, "@policy_deny drop") {
		t.Fatal("missing deny rule")
	}
	if !strings.Contains(out, "@policy_allow accept") {
		t.Fatal("missing allow rule")
	}
}

func TestRenderPolicyNftWildcardSkipped(t *testing.T) {
	rules := []policy.Rule{
		{PolicyID: "p1", AgentID: "*", Domains: []string{"*"}, Action: "allow"},
	}
	out := RenderPolicyNft(rules, "", "")
	// Wildcard "*" should not produce any set elements.
	if strings.Contains(out, "elements") {
		t.Fatal("wildcard * should not produce set elements")
	}
}

func TestRenderPolicyNftEmpty(t *testing.T) {
	out := RenderPolicyNft(nil, "test", "chain1")
	if !strings.Contains(out, "table inet test") {
		t.Fatal("should still produce table")
	}
	if !strings.Contains(out, "chain chain1") {
		t.Fatal("should still produce chain")
	}
}

func TestRenderPolicyNftDedup(t *testing.T) {
	rules := []policy.Rule{
		{PolicyID: "p1", AgentID: "*", Domains: []string{"127.0.0.1"}, Action: "allow"},
		{PolicyID: "p2", AgentID: "a1", Domains: []string{"127.0.0.1"}, Action: "allow"},
	}
	out := RenderPolicyNft(rules, "", "")
	// Should only have one 127.0.0.1 in the set.
	count := strings.Count(out, "127.0.0.1")
	if count != 1 {
		t.Fatalf("expected 1 occurrence of 127.0.0.1, got %d", count)
	}
}
