package policy

import "fmt"

// Conflict describes two rules that match the same (agent, domain) pair
// but produce different actions.
type Conflict struct {
	RuleA    Rule   `json:"rule_a"`
	RuleB    Rule   `json:"rule_b"`
	Domain   string `json:"domain"`
	AgentID  string `json:"agent_id"`
	Severity string `json:"severity"` // "warning" (same priority) or "shadowed" (B never fires)
}

// DetectConflicts scans the rule list for overlapping matches with different actions.
// Rules are evaluated in order (first match wins), so a later rule with the same
// match criteria but a different action is "shadowed" — it will never fire.
func DetectConflicts(rules []Rule) []Conflict {
	var conflicts []Conflict

	for i := 0; i < len(rules); i++ {
		for j := i + 1; j < len(rules); j++ {
			a, b := rules[i], rules[j]
			if a.Action == b.Action {
				continue // same action = no conflict
			}
			// Check agent overlap.
			if !agentOverlaps(a.AgentID, b.AgentID) {
				continue
			}
			// Check domain overlap.
			for _, da := range a.Domains {
				for _, db := range b.Domains {
					if domainOverlaps(da, db) {
						agentDesc := a.AgentID
						if agentDesc == "*" || b.AgentID == "*" {
							agentDesc = "*"
						}
						conflicts = append(conflicts, Conflict{
							RuleA:    a,
							RuleB:    b,
							Domain:   fmt.Sprintf("%s / %s", da, db),
							AgentID:  agentDesc,
							Severity: "shadowed",
						})
					}
				}
			}
		}
	}
	return conflicts
}

func agentOverlaps(a, b string) bool {
	return a == "*" || b == "*" || a == b
}

func domainOverlaps(a, b string) bool {
	if a == "*" || b == "*" {
		return true
	}
	if a == b {
		return true
	}
	// *.example.com overlaps with example.com
	if matchDomain(a, b) || matchDomain(b, a) {
		return true
	}
	return false
}
