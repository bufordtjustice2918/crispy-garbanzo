package cmdmap

import "strings"

type Command struct {
	Kind        string `json:"kind"`
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
	Backend     string `json:"backend"`
	Example     string `json:"example"`
}

var SetCommands = []Command{
	{Kind: "set", Pattern: "system host-name <name>", Description: "Set system hostname", Backend: "systemd-hostnamed", Example: "set system host-name clawgress-gw"},
	{Kind: "set", Pattern: "system ntp server <server>", Description: "Add NTP server", Backend: "chrony/ntp config", Example: "set system ntp server 0.pool.ntp.org"},
	{Kind: "set", Pattern: "interfaces ethernet <ifname> address <cidr>", Description: "Assign interface address", Backend: "systemd-networkd", Example: "set interfaces ethernet eth0 address 192.168.10.1/24"},
	{Kind: "set", Pattern: "interfaces ethernet <ifname> role <lan|wan>", Description: "Mark interface role", Backend: "gateway profile renderer", Example: "set interfaces ethernet eth0 role lan"},
	{Kind: "set", Pattern: "firewall nftables input default-action <accept|drop>", Description: "Set input policy", Backend: "nftables ruleset", Example: "set firewall nftables input default-action drop"},
	{Kind: "set", Pattern: "firewall nftables forward default-action <accept|drop>", Description: "Set forward policy", Backend: "nftables ruleset", Example: "set firewall nftables forward default-action drop"},
	{Kind: "set", Pattern: "firewall nftables wan-block enable", Description: "Enable default WAN block posture", Backend: "nftables ruleset", Example: "set firewall nftables wan-block enable"},
	{Kind: "set", Pattern: "firewall group address-group <name> address <cidr>", Description: "Define reusable address group", Backend: "nftables sets", Example: "set firewall group address-group blocked-nets address 203.0.113.0/24"},
	{Kind: "set", Pattern: "nat source rule <id> outbound-interface <ifname>", Description: "Set NAT outbound interface", Backend: "nftables nat chain", Example: "set nat source rule 100 outbound-interface eth1"},
	{Kind: "set", Pattern: "nat source rule <id> source address <cidr>", Description: "Set NAT source match", Backend: "nftables nat chain", Example: "set nat source rule 100 source address 10.0.0.0/8"},
	{Kind: "set", Pattern: "nat source rule <id> translation address masquerade", Description: "Enable source NAT masquerade", Backend: "nftables nat chain", Example: "set nat source rule 100 translation address masquerade"},
	{Kind: "set", Pattern: "service dns forwarding listen-address <ip>", Description: "DNS listener address", Backend: "bind9 named options", Example: "set service dns forwarding listen-address 0.0.0.0"},
	{Kind: "set", Pattern: "service dns forwarding allow-from <cidr>", Description: "DNS recursion allowlist", Backend: "bind9 ACL", Example: "set service dns forwarding allow-from 10.0.0.0/8"},
	{Kind: "set", Pattern: "service haproxy enable", Description: "Enable HAProxy service", Backend: "systemd unit", Example: "set service haproxy enable"},
	{Kind: "set", Pattern: "service haproxy stats port <port>", Description: "Set HAProxy stats port", Backend: "haproxy.cfg", Example: "set service haproxy stats port 8404"},
	{Kind: "set", Pattern: "policy egress default-action <allow|deny>", Description: "Set default egress action", Backend: "policy evaluator", Example: "set policy egress default-action deny"},
	{Kind: "set", Pattern: "policy egress allow-domain <fqdn>", Description: "Allow egress domain", Backend: "policy + nft set/domain resolver", Example: "set policy egress allow-domain api.openai.com"},
	{Kind: "set", Pattern: "policy egress deny-domain <fqdn>", Description: "Deny egress domain", Backend: "policy + nft set/domain resolver", Example: "set policy egress deny-domain example.com"},
}

var ShowCommands = []Command{
	{Kind: "show", Pattern: "show commands", Description: "List supported command mappings", Backend: "command catalog", Example: "show commands"},
	{Kind: "show", Pattern: "show configuration", Description: "Render candidate config as JSON", Backend: "candidate store", Example: "show configuration"},
	{Kind: "show", Pattern: "show configuration commands", Description: "Render candidate as set commands", Backend: "candidate store", Example: "show configuration commands"},
	{Kind: "show", Pattern: "show system ntp", Description: "Show configured NTP servers", Backend: "candidate/active config", Example: "show system ntp"},
	{Kind: "show", Pattern: "show interfaces", Description: "Show interface config", Backend: "candidate/active config", Example: "show interfaces"},
	{Kind: "show", Pattern: "show firewall", Description: "Show firewall config", Backend: "candidate/active config", Example: "show firewall"},
	{Kind: "show", Pattern: "show nat source rules", Description: "Show NAT source rules", Backend: "candidate/active config", Example: "show nat source rules"},
	{Kind: "show", Pattern: "show service dns", Description: "Show DNS forwarding config", Backend: "candidate/active config", Example: "show service dns"},
	{Kind: "show", Pattern: "show service haproxy", Description: "Show HAProxy config", Backend: "candidate/active config", Example: "show service haproxy"},
}

func Commands() map[string][]Command {
	return map[string][]Command{"set": SetCommands, "show": ShowCommands}
}

func MatchSet(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	tokens := normalizeTokens(strings.Split(path, "."))
	for _, p := range TokenPaths {
		if p.Kind != "set" {
			continue
		}
		if matchPatternTokens(normalizeTokens(p.Tokens), tokens) {
			return true
		}
	}
	return false
}

func patternToPath(pattern string) string {
	return strings.Join(strings.Fields(pattern), ".")
}

func matchPatternTokens(pattern, value []string) bool {
	if len(pattern) != len(value) {
		return false
	}
	for i := range pattern {
		pt := pattern[i]
		vt := value[i]
		if strings.HasPrefix(pt, "<") && strings.HasSuffix(pt, ">") {
			if vt == "" {
				return false
			}
			continue
		}
		if strings.Contains(pt, "|") {
			alts := strings.Split(strings.Trim(pt, "<>"), "|")
			ok := false
			for _, alt := range alts {
				if vt == alt {
					ok = true
					break
				}
			}
			if !ok {
				return false
			}
			continue
		}
		if pt != vt {
			return false
		}
	}
	return true
}

func normalizeTokens(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, strings.ReplaceAll(t, "-", "_"))
	}
	return out
}
