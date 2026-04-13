package policy

import "testing"

// BenchmarkEvaluateHit benchmarks policy evaluation with a direct match.
func BenchmarkEvaluateHit(b *testing.B) {
	eng := &Engine{}
	eng.rules = []Rule{
		{PolicyID: "p1", AgentID: "a1", Domains: []string{"example.com"}, Action: "allow"},
		{PolicyID: "p2", AgentID: "*", Domains: []string{"*.test.com"}, Action: "deny"},
		{PolicyID: "p3", AgentID: "*", Domains: []string{"*"}, Action: "deny"},
	}
	b.ResetTimer()
	for range b.N {
		eng.Evaluate("a1", "example.com")
	}
}

// BenchmarkEvaluateWildcard benchmarks wildcard domain matching.
func BenchmarkEvaluateWildcard(b *testing.B) {
	eng := &Engine{}
	eng.rules = []Rule{
		{PolicyID: "p1", AgentID: "*", Domains: []string{"*.example.com"}, Action: "allow"},
		{PolicyID: "p2", AgentID: "*", Domains: []string{"*"}, Action: "deny"},
	}
	b.ResetTimer()
	for range b.N {
		eng.Evaluate("a1", "sub.deep.example.com:443")
	}
}

// BenchmarkEvaluateDefaultDeny benchmarks the worst case: scan all rules, no match.
func BenchmarkEvaluateDefaultDeny(b *testing.B) {
	var rules []Rule
	for i := range 100 {
		rules = append(rules, Rule{
			PolicyID: "p" + string(rune(i)),
			AgentID:  "other",
			Domains:  []string{"domain" + string(rune(i)) + ".com"},
			Action:   "allow",
		})
	}
	eng := &Engine{}
	eng.rules = rules
	b.ResetTimer()
	for range b.N {
		eng.Evaluate("a1", "nomatch.com")
	}
}
