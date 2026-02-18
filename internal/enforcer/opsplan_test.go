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
	op, ok := findOp(ops, "dhclient+systemd", "interfaces ethernet eth0 address dhcp")
	if !ok {
		t.Fatalf("expected dhclient op")
	}
	if len(op.Actions) < 4 {
		t.Fatalf("expected dhcp actions, got %v", op.Actions)
	}
}

func TestBuildOpsPlanFirewallNatPolicy(t *testing.T) {
	changes := map[string]any{
		"firewall": map[string]any{
			"nftables": map[string]any{"input": map[string]any{"default_action": "drop"}},
		},
		"nat": map[string]any{
			"source": map[string]any{"rule": map[string]any{"100": map[string]any{"translation": map[string]any{"address": "masquerade"}}}},
		},
		"policy": map[string]any{
			"egress": map[string]any{"default_action": "deny"},
		},
	}

	ops := BuildOpsPlan(changes)
	if _, ok := findOp(ops, "nftables", "firewall"); !ok {
		t.Fatalf("expected firewall nftables op")
	}
	if _, ok := findOp(ops, "nftables", "nat"); !ok {
		t.Fatalf("expected nat nftables op")
	}
	if _, ok := findOp(ops, "clawgress-policyd", "policy"); !ok {
		t.Fatalf("expected policy daemon op")
	}
}

func TestBuildOpsPlanGenericServiceFallback(t *testing.T) {
	changes := map[string]any{
		"service": map[string]any{
			"dns":         map[string]any{"forwarding": map[string]any{"listen_address": []any{"10.0.0.1"}}},
			"dhcp_relay":  map[string]any{"interface": "eth0"},
			"snmp_server": map[string]any{"contact": "ops@example.com"},
		},
	}
	ops := BuildOpsPlan(changes)
	if _, ok := findOp(ops, "bind9", "service dns forwarding"); !ok {
		t.Fatalf("expected bind9 op")
	}
	if _, ok := findOp(ops, "systemd-unit", "service dhcp-relay"); !ok {
		t.Fatalf("expected dhcp-relay unit op")
	}
	if _, ok := findOp(ops, "systemd-unit", "service snmp-server"); !ok {
		t.Fatalf("expected snmp-server unit op")
	}
}

func findOp(ops []Operation, backend, scope string) (Operation, bool) {
	for _, op := range ops {
		if op.Backend == backend && op.Scope == scope {
			return op, true
		}
	}
	return Operation{}, false
}
