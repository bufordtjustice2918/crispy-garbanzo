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
	return ops
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
			out = append(out, Operation{
				Backend: "dhclient+systemd",
				Scope:   fmt.Sprintf("interfaces ethernet %s address dhcp", ifname),
				Actions: []string{
					fmt.Sprintf("write /run/dhclient/dhclient_%s.conf", ifname),
					fmt.Sprintf("write /run/systemd/system/dhclient@%s.service.d/10-override.conf", ifname),
					"systemctl daemon-reload",
					fmt.Sprintf("systemctl restart dhclient@%s.service", ifname),
				},
			})
			continue
		}
		addrs := collectAddresses(addrRaw)
		if len(addrs) == 0 {
			continue
		}
		out = append(out, Operation{
			Backend: "systemd-networkd",
			Scope:   fmt.Sprintf("interfaces ethernet %s address", ifname),
			Actions: []string{
				fmt.Sprintf("write /etc/systemd/network/10-%s.network (Address=%s)", ifname, strings.Join(addrs, ",")),
				"systemctl restart systemd-networkd",
			},
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
