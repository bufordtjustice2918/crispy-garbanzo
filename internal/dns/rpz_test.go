package dns

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bufordtjustice2918/crispy-garbanzo/internal/policy"
)

func TestGenerateRPZBasic(t *testing.T) {
	rules := []policy.Rule{
		{PolicyID: "allow-good", AgentID: "*", Domains: []string{"good.com"}, Action: "allow"},
		{PolicyID: "deny-evil", AgentID: "*", Domains: []string{"evil.com"}, Action: "deny"},
		{PolicyID: "deny-bad", AgentID: "*", Domains: []string{"bad.org"}, Action: "deny"},
	}

	out := GenerateRPZ(rules, RPZConfig{Serial: 1})

	// Should contain deny entries.
	if !strings.Contains(out, "evil.com") {
		t.Fatal("missing evil.com")
	}
	if !strings.Contains(out, "bad.org") {
		t.Fatal("missing bad.org")
	}
	if !strings.Contains(out, "CNAME .") {
		t.Fatal("missing CNAME . records")
	}

	// Should NOT contain allow entries.
	if strings.Contains(out, "good.com") {
		t.Fatal("allow domain should not be in RPZ")
	}
}

func TestGenerateRPZWildcard(t *testing.T) {
	rules := []policy.Rule{
		{PolicyID: "deny-wild", AgentID: "*", Domains: []string{"*.evil.com"}, Action: "deny"},
	}

	out := GenerateRPZ(rules, RPZConfig{Serial: 1})

	// Should have both base and wildcard entries.
	if !strings.Contains(out, "evil.com") {
		t.Fatal("missing base domain for wildcard")
	}
	if !strings.Contains(out, "*.evil.com") {
		t.Fatal("missing wildcard entry")
	}
}

func TestGenerateRPZDedup(t *testing.T) {
	rules := []policy.Rule{
		{PolicyID: "p1", AgentID: "a1", Domains: []string{"evil.com"}, Action: "deny"},
		{PolicyID: "p2", AgentID: "a2", Domains: []string{"evil.com"}, Action: "deny"},
	}

	out := GenerateRPZ(rules, RPZConfig{Serial: 1})
	count := strings.Count(out, "evil.com")
	// Should appear exactly once (plus possibly in comments).
	lines := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "evil.com") && strings.Contains(line, "CNAME") {
			lines++
		}
	}
	if lines != 1 {
		t.Fatalf("evil.com should appear once, got %d times (total mentions: %d)", lines, count)
	}
}

func TestGenerateRPZStarSkipped(t *testing.T) {
	rules := []policy.Rule{
		{PolicyID: "p1", AgentID: "*", Domains: []string{"*"}, Action: "deny"},
	}

	out := GenerateRPZ(rules, RPZConfig{Serial: 1})
	if strings.Contains(out, "CNAME .") {
		t.Fatal("wildcard * should not produce RPZ entries")
	}
}

func TestGenerateRPZSOA(t *testing.T) {
	out := GenerateRPZ(nil, RPZConfig{
		ZoneName:   "test.rpz",
		SOAContact: "hostmaster.test",
		Serial:     12345,
		TTL:        30,
	})

	if !strings.Contains(out, "test.rpz") {
		t.Fatal("missing zone name in SOA")
	}
	if !strings.Contains(out, "hostmaster.test") {
		t.Fatal("missing SOA contact")
	}
	if !strings.Contains(out, "12345") {
		t.Fatal("missing serial")
	}
	if !strings.Contains(out, "$TTL 30") {
		t.Fatal("missing TTL")
	}
}

func TestWriteRPZFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db.rpz")

	rules := []policy.Rule{
		{PolicyID: "p1", AgentID: "*", Domains: []string{"blocked.com", "also-blocked.net"}, Action: "deny"},
		{PolicyID: "p2", AgentID: "*", Domains: []string{"ok.com"}, Action: "allow"},
	}

	result, err := WriteRPZFile(path, rules, RPZConfig{Serial: 99})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeniedCount != 2 {
		t.Fatalf("expected 2 denied, got %d", result.DeniedCount)
	}

	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		t.Fatal("file is empty")
	}
}

func TestGenerateNamedConf(t *testing.T) {
	out := GenerateNamedConf("rpz.test", "/etc/bind/db.rpz.test")
	if !strings.Contains(out, `zone "rpz.test"`) {
		t.Fatal("missing zone declaration")
	}
	if !strings.Contains(out, "/etc/bind/db.rpz.test") {
		t.Fatal("missing zone file path")
	}
	if !strings.Contains(out, "response-policy") {
		t.Fatal("missing response-policy directive")
	}
}
