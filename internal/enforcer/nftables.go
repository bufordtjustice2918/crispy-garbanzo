package enforcer

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bufordtjustice2918/crispy-garbanzo/internal/opmode"
)

type ApplyResult struct {
	RulesPath string `json:"rules_path"`
	Applied   bool   `json:"applied"`
}

func ApplyNftables(baseDir string, rev opmode.Revision, execute bool) (ApplyResult, error) {
	rules := renderRules(rev.Changes)

	compiledDir := filepath.Join(baseDir, "compiled")
	if err := os.MkdirAll(compiledDir, 0o755); err != nil {
		return ApplyResult{}, fmt.Errorf("create compiled dir: %w", err)
	}
	rulesPath := filepath.Join(compiledDir, "nftables-active.nft")
	if err := os.WriteFile(rulesPath, []byte(rules), 0o644); err != nil {
		return ApplyResult{}, fmt.Errorf("write rules file: %w", err)
	}

	result := ApplyResult{RulesPath: rulesPath, Applied: false}
	if !execute {
		return result, nil
	}

	_ = exec.Command("nft", "delete", "table", "inet", "clawgress_dynamic").Run()
	_ = exec.Command("nft", "delete", "table", "ip", "clawgress_dynamic_nat").Run()

	cmd := exec.Command("nft", "-f", rulesPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return result, fmt.Errorf("apply nftables rules: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	result.Applied = true
	return result, nil
}

func renderRules(changes map[string]any) string {
	inputPolicy := toNftPolicy(getString(changes, "firewall", "nftables", "input", "default_action"), "drop")
	forwardPolicy := toNftPolicy(getString(changes, "firewall", "nftables", "forward", "default_action"), "drop")
	outputPolicy := toNftPolicy(getString(changes, "firewall", "nftables", "output", "default_action"), "accept")

	wanBlock := getBool(changes, "firewall", "nftables", "wan_block")
	wanIfaces := getWanIfaces(changes)

	var b bytes.Buffer
	b.WriteString("#!/usr/sbin/nft -f\n\n")
	b.WriteString("table inet clawgress_dynamic {\n")
	if wanBlock && len(wanIfaces) > 0 {
		b.WriteString("  set wan_ifaces {\n")
		b.WriteString("    type ifname\n")
		b.WriteString("    elements = { ")
		for i, ifn := range wanIfaces {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("\"%s\"", ifn))
		}
		b.WriteString(" }\n")
		b.WriteString("  }\n\n")
	}

	b.WriteString(fmt.Sprintf("  chain input {\n    type filter hook input priority 0; policy %s;\n", inputPolicy))
	b.WriteString("    iif lo accept\n")
	b.WriteString("    ct state established,related accept\n")
	if wanBlock && len(wanIfaces) > 0 {
		b.WriteString("    iifname @wan_ifaces drop\n")
	}
	b.WriteString("    tcp dport { 22, 53, 80, 443, 8404 } accept\n")
	b.WriteString("    udp dport { 53 } accept\n")
	b.WriteString("  }\n\n")

	b.WriteString(fmt.Sprintf("  chain forward {\n    type filter hook forward priority 0; policy %s;\n", forwardPolicy))
	b.WriteString("    ct state established,related accept\n")
	b.WriteString("  }\n\n")

	b.WriteString(fmt.Sprintf("  chain output {\n    type filter hook output priority 0; policy %s;\n  }\n", outputPolicy))
	b.WriteString("}\n")

	natRules := renderSourceNatRules(changes)
	if len(natRules) > 0 {
		b.WriteString("\n")
		b.WriteString("table ip clawgress_dynamic_nat {\n")
		b.WriteString("  chain postrouting {\n")
		b.WriteString("    type nat hook postrouting priority srcnat; policy accept;\n")
		for _, r := range natRules {
			b.WriteString("    ")
			b.WriteString(r)
			b.WriteString("\n")
		}
		b.WriteString("  }\n")
		b.WriteString("}\n")
	}

	return b.String()
}

func renderSourceNatRules(changes map[string]any) []string {
	rulesMap := mapAt(changes, "nat", "source", "rule")
	if len(rulesMap) == 0 {
		return nil
	}
	ids := make([]string, 0, len(rulesMap))
	for id := range rulesMap {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]string, 0, len(ids))
	for _, id := range ids {
		rule, ok := rulesMap[id].(map[string]any)
		if !ok {
			continue
		}
		oif := toString(rule["outbound_interface"])
		src := getString(rule, "source", "address")
		xlat := getString(rule, "translation", "address")
		if oif == "" || src == "" || strings.ToLower(xlat) != "masquerade" {
			continue
		}
		out = append(out, fmt.Sprintf("oifname \"%s\" ip saddr %s masquerade", oif, src))
	}
	return out
}

func getWanIfaces(changes map[string]any) []string {
	eth := mapAt(changes, "interfaces", "ethernet")
	if len(eth) == 0 {
		return nil
	}
	ifaces := make([]string, 0)
	for ifn, raw := range eth {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if strings.EqualFold(toString(m["role"]), "wan") {
			ifaces = append(ifaces, ifn)
		}
	}
	sort.Strings(ifaces)
	return ifaces
}

func toNftPolicy(v, fallback string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "accept", "drop":
		return v
	default:
		return fallback
	}
}

func getString(root map[string]any, path ...string) string {
	cur := any(root)
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		next, ok := m[p]
		if !ok {
			return ""
		}
		cur = next
	}
	return toString(cur)
}

func getBool(root map[string]any, path ...string) bool {
	cur := any(root)
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		next, ok := m[p]
		if !ok {
			return false
		}
		cur = next
	}
	b, ok := cur.(bool)
	return ok && b
}

func mapAt(root map[string]any, path ...string) map[string]any {
	cur := any(root)
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		next, ok := m[p]
		if !ok {
			return nil
		}
		cur = next
	}
	m, ok := cur.(map[string]any)
	if !ok {
		return nil
	}
	return m
}

func toString(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
