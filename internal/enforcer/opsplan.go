package enforcer

import (
	"fmt"
	"sort"
	"strings"
)

type Operation struct {
	Backend string   `json:"backend"`
	Scope   string   `json:"scope"`
	Actions []string `json:"actions"`
}

func BuildOpsPlan(changes map[string]any) []Operation {
	ops := make([]Operation, 0)
	ops = append(ops, buildInterfaceOps(changes)...)
	ops = append(ops, buildSystemOps(changes)...)
	ops = append(ops, buildServiceOps(changes)...)
	ops = append(ops, buildFirewallNatOps(changes)...)
	ops = append(ops, buildPolicyOps(changes)...)
	ops = append(ops, buildFamilyFallbackOps(changes)...)
	return dedupeOps(ops)
}

func buildInterfaceOps(changes map[string]any) []Operation {
	eth := mapAt(changes, "interfaces", "ethernet")
	if len(eth) == 0 {
		return nil
	}
	ifnames := make([]string, 0, len(eth))
	for ifname := range eth {
		ifnames = append(ifnames, ifname)
	}
	sort.Strings(ifnames)

	out := make([]Operation, 0)
	for _, ifname := range ifnames {
		m, ok := eth[ifname].(map[string]any)
		if !ok {
			continue
		}
		addrRaw := m["address"]
		if hasDHCPAddress(addrRaw) {
			actions := []string{
				fmt.Sprintf("write /run/dhclient/dhclient_%s.conf", ifname),
				fmt.Sprintf("write /run/systemd/system/dhclient@%s.service.d/10-override.conf", ifname),
				"systemctl daemon-reload",
				fmt.Sprintf("systemctl restart dhclient@%s.service", ifname),
			}
			actions = append(actions, interfaceStateActions(ifname, m)...)
			out = append(out, Operation{
				Backend: "dhclient+systemd",
				Scope:   fmt.Sprintf("interfaces ethernet %s address dhcp", ifname),
				Actions: actions,
			})
			continue
		}
		addrs := collectAddresses(addrRaw)
		actions := interfaceStateActions(ifname, m)
		if len(addrs) == 0 && len(actions) == 0 {
			continue
		}
		if len(addrs) > 0 {
			actions = append([]string{
				fmt.Sprintf("write /etc/systemd/network/10-%s.network (Address=%s)", ifname, strings.Join(addrs, ",")),
				"systemctl restart systemd-networkd",
			}, actions...)
		}
		out = append(out, Operation{
			Backend: "systemd-networkd",
			Scope:   fmt.Sprintf("interfaces ethernet %s", ifname),
			Actions: actions,
		})
	}
	return out
}

func buildSystemOps(changes map[string]any) []Operation {
	out := make([]Operation, 0)
	if h := getString(changes, "system", "host_name"); h != "" {
		out = append(out, Operation{
			Backend: "hostnamectl",
			Scope:   "system host-name",
			Actions: []string{fmt.Sprintf("hostnamectl set-hostname %s", h)},
		})
	}

	nameServers := collectAddresses(getAt(changes, "system", "name_server"))
	if len(nameServers) > 0 {
		out = append(out, Operation{
			Backend: "systemd-resolved",
			Scope:   "system name-server",
			Actions: []string{
				fmt.Sprintf("write /etc/systemd/resolved.conf.d/90-clawgress.conf (DNS=%s)", strings.Join(nameServers, " ")),
				"systemctl restart systemd-resolved",
			},
		})
	}

	ntpServers := collectAddresses(getAt(changes, "system", "ntp", "server"))
	if len(ntpServers) > 0 {
		out = append(out, Operation{
			Backend: "chrony",
			Scope:   "system ntp server",
			Actions: []string{
				fmt.Sprintf("write /etc/chrony/sources.d/clawgress.sources (%s)", strings.Join(ntpServers, ",")),
				"systemctl restart chrony",
			},
		})
	}
	return out
}

func buildServiceOps(changes map[string]any) []Operation {
	out := make([]Operation, 0)
	service := mapAt(changes, "service")
	if len(service) == 0 {
		return out
	}

	dns := mapAt(changes, "service", "dns", "forwarding")
	if len(dns) > 0 {
		out = append(out, Operation{
			Backend: "bind9",
			Scope:   "service dns forwarding",
			Actions: []string{
				"write /etc/bind/named.conf.options",
				"systemctl restart bind9",
			},
		})
	}
	haproxy := mapAt(changes, "service", "haproxy")
	if len(haproxy) > 0 {
		out = append(out, Operation{
			Backend: "haproxy",
			Scope:   "service haproxy",
			Actions: []string{
				"write /etc/haproxy/haproxy.cfg",
				"systemctl restart haproxy",
			},
		})
	}
	ssh := mapAt(changes, "service", "ssh")
	if len(ssh) > 0 {
		out = append(out, Operation{
			Backend: "openssh-server",
			Scope:   "service ssh",
			Actions: []string{
				"write /etc/ssh/sshd_config.d/90-clawgress.conf",
				"systemctl restart ssh",
			},
		})
	}
	for name, raw := range service {
		if name == "dns" || name == "haproxy" || name == "ssh" {
			continue
		}
		if _, ok := raw.(map[string]any); !ok {
			continue
		}
		unit := strings.ReplaceAll(name, "_", "-")
		out = append(out, Operation{
			Backend: "systemd-unit",
			Scope:   fmt.Sprintf("service %s", strings.ReplaceAll(name, "_", "-")),
			Actions: []string{
				fmt.Sprintf("write /etc/clawgress/services/%s.json", name),
				fmt.Sprintf("systemctl reload-or-restart %s", unit),
			},
		})
	}
	return out
}

func buildFirewallNatOps(changes map[string]any) []Operation {
	out := make([]Operation, 0)
	if len(mapAt(changes, "firewall")) > 0 {
		out = append(out, Operation{
			Backend: "nftables",
			Scope:   "firewall",
			Actions: []string{
				"write /etc/nftables.d/40-clawgress-firewall.nft",
				"nft -c -f /etc/nftables.d/40-clawgress-firewall.nft",
				"nft -f /etc/nftables.d/40-clawgress-firewall.nft",
			},
		})
	}
	if len(mapAt(changes, "nat")) > 0 {
		out = append(out, Operation{
			Backend: "nftables",
			Scope:   "nat",
			Actions: []string{
				"write /etc/nftables.d/50-clawgress-nat.nft",
				"nft -c -f /etc/nftables.d/50-clawgress-nat.nft",
				"nft -f /etc/nftables.d/50-clawgress-nat.nft",
			},
		})
	}
	return out
}

func buildPolicyOps(changes map[string]any) []Operation {
	if len(mapAt(changes, "policy")) == 0 {
		return nil
	}
	return []Operation{
		{
			Backend: "clawgress-policyd",
			Scope:   "policy",
			Actions: []string{
				"write /etc/clawgress/policy/policy.json",
				"systemctl reload-or-restart clawgress-policyd",
			},
		},
	}
}

func buildFamilyFallbackOps(changes map[string]any) []Operation {
	roots := []string{"system", "interfaces", "service", "firewall", "nat", "policy"}
	out := make([]Operation, 0)
	for _, root := range roots {
		m := mapAt(changes, root)
		if len(m) == 0 {
			continue
		}
		out = append(out, Operation{
			Backend: "control-daemon",
			Scope:   root,
			Actions: []string{
				fmt.Sprintf("write /etc/clawgress/rendered/%s.json", root),
				"systemctl reload-or-restart clawgress-configd",
			},
		})
	}
	return out
}

func getAt(root map[string]any, path ...string) any {
	cur := any(root)
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		v, ok := m[p]
		if !ok {
			return nil
		}
		cur = v
	}
	return cur
}

func hasDHCPAddress(v any) bool {
	switch t := v.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "dhcp")
	case []any:
		for _, item := range t {
			s, ok := item.(string)
			if ok && strings.EqualFold(strings.TrimSpace(s), "dhcp") {
				return true
			}
		}
	}
	return false
}

func collectAddresses(v any) []string {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		return []string{s}
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s, ok := item.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		return out
	default:
		return nil
	}
}

func interfaceStateActions(ifname string, m map[string]any) []string {
	out := make([]string, 0)
	if desc := toString(m["description"]); desc != "" {
		out = append(out, fmt.Sprintf("write /run/systemd/network/10-%s.link (Description=%s)", ifname, desc))
	}
	switch mtu := m["mtu"].(type) {
	case float64:
		out = append(out, fmt.Sprintf("ip link set dev %s mtu %d", ifname, int(mtu)))
	case int:
		out = append(out, fmt.Sprintf("ip link set dev %s mtu %d", ifname, mtu))
	case string:
		if strings.TrimSpace(mtu) != "" {
			out = append(out, fmt.Sprintf("ip link set dev %s mtu %s", ifname, mtu))
		}
	}
	if role := toString(m["role"]); role != "" {
		out = append(out, fmt.Sprintf("write /etc/clawgress/interface-role.d/%s.role (%s)", ifname, role))
	}
	if v, ok := m["disable"].(bool); ok && v {
		out = append(out, fmt.Sprintf("ip link set dev %s down", ifname))
	}
	return out
}

func dedupeOps(in []Operation) []Operation {
	out := make([]Operation, 0, len(in))
	seen := map[string]bool{}
	for _, op := range in {
		if op.Backend == "" || op.Scope == "" {
			continue
		}
		key := op.Backend + "|" + op.Scope
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, op)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Backend != out[j].Backend {
			return out[i].Backend < out[j].Backend
		}
		return out[i].Scope < out[j].Scope
	})
	return out
}
