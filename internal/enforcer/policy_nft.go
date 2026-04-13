package enforcer

import (
	"fmt"
	"net"
	"strings"

	"github.com/bufordtjustice2918/crispy-garbanzo/internal/policy"
)

// RenderPolicyNft generates an nftables ruleset fragment from policy rules.
// It creates a named set of allowed domains (resolved to IPs where possible)
// and a set of denied domains, then emits rules that use these sets.
//
// The output is suitable for inclusion in /etc/nftables.conf or atomic
// application via `nft -f`.
func RenderPolicyNft(rules []policy.Rule, tableName, chainName string) string {
	if tableName == "" {
		tableName = "clawgress"
	}
	if chainName == "" {
		chainName = "egress_policy"
	}

	var allowIPs, denyIPs []string

	for _, r := range rules {
		for _, d := range r.Domains {
			if d == "*" {
				continue // wildcard handled by default chain policy
			}
			ips := resolveDomain(d)
			switch r.Action {
			case "allow":
				allowIPs = append(allowIPs, ips...)
			case "deny":
				denyIPs = append(denyIPs, ips...)
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Auto-generated from policy engine — do not edit\n"))
	sb.WriteString(fmt.Sprintf("table inet %s {\n", tableName))

	// Deny set.
	if len(denyIPs) > 0 {
		sb.WriteString(fmt.Sprintf("  set policy_deny {\n    type ipv4_addr\n    elements = { %s }\n  }\n",
			strings.Join(dedup(denyIPs), ", ")))
	}

	// Allow set.
	if len(allowIPs) > 0 {
		sb.WriteString(fmt.Sprintf("  set policy_allow {\n    type ipv4_addr\n    elements = { %s }\n  }\n",
			strings.Join(dedup(allowIPs), ", ")))
	}

	sb.WriteString(fmt.Sprintf("  chain %s {\n", chainName))
	if len(denyIPs) > 0 {
		sb.WriteString("    ip daddr @policy_deny drop\n")
	}
	if len(allowIPs) > 0 {
		sb.WriteString("    ip daddr @policy_allow accept\n")
	}
	sb.WriteString("  }\n")
	sb.WriteString("}\n")

	return sb.String()
}

// resolveDomain attempts to resolve a domain to IP addresses.
// For wildcard patterns (*.example.com), it tries resolving the base domain.
// Returns the domain itself if resolution fails (useful for logging).
func resolveDomain(domain string) []string {
	// Strip wildcard prefix.
	lookup := domain
	if strings.HasPrefix(domain, "*.") {
		lookup = domain[2:]
	}

	// Skip obviously invalid domains.
	if strings.HasSuffix(lookup, ".invalid") || strings.HasSuffix(lookup, ".local") {
		return nil
	}

	// If it's already an IP, return it directly.
	if ip := net.ParseIP(lookup); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			return []string{ip4.String()}
		}
		return nil // skip IPv6 for now
	}

	addrs, err := net.LookupHost(lookup)
	if err != nil {
		return nil
	}

	var ips []string
	for _, a := range addrs {
		if ip := net.ParseIP(a); ip != nil {
			if ip4 := ip.To4(); ip4 != nil {
				ips = append(ips, ip4.String())
			}
		}
	}
	return ips
}

func dedup(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
