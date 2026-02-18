package enforcer

import "testing"

func TestBuildOpsPlanInterfaceDHCP(t *testing.T) {
	changes := map[string]any{
		"interfaces": map[string]any{
			"ethernet": map[string]any{
				"eth0": map[string]any{
					"address": "dhcp",
				},
			},
		},
	}

	ops := BuildOpsPlan(changes)
	if len(ops) == 0 {
		t.Fatalf("expected operations")
	}
	if ops[0].Backend != "dhclient+systemd" {
		t.Fatalf("expected dhclient+systemd backend, got %q", ops[0].Backend)
	}
	if ops[0].Scope != "interfaces ethernet eth0 address dhcp" {
		t.Fatalf("unexpected scope: %q", ops[0].Scope)
	}
}
