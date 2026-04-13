package quota

import "testing"

// BenchmarkCheckNoLimit benchmarks the fast path: no quota configured.
func BenchmarkCheckNoLimit(b *testing.B) {
	lim, _ := NewLimiter("/nonexistent")
	b.ResetTimer()
	for range b.N {
		lim.Check("unknown-agent")
	}
}

// BenchmarkCheckWithLimit benchmarks quota check with a configured limit.
func BenchmarkCheckWithLimit(b *testing.B) {
	lim, _ := NewLimiter("/nonexistent")
	lim.Set(Limit{AgentID: "a1", RPS: 1000000, Mode: "hard_stop"})
	b.ResetTimer()
	for range b.N {
		lim.Check("a1")
	}
}
