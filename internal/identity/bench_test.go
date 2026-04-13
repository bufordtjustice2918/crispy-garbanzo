package identity

import "testing"

// BenchmarkLookupByKey benchmarks the hot path: API key lookup.
func BenchmarkLookupByKey(b *testing.B) {
	reg := &Registry{
		byKey: make(map[string]*Agent),
		byID:  make(map[string]*Agent),
	}
	for i := range 1000 {
		a := &Agent{
			AgentID: "agent-" + string(rune(i)),
			APIKey:  "key-" + string(rune(i)),
			Status:  "active",
		}
		reg.byKey[a.APIKey] = a
		reg.byID[a.AgentID] = a
	}
	// Lookup a key that exists.
	target := &Agent{AgentID: "target", APIKey: "target-key", Status: "active"}
	reg.byKey[target.APIKey] = target
	reg.byID[target.AgentID] = target

	b.ResetTimer()
	for range b.N {
		reg.LookupByKey("target-key")
	}
}

// BenchmarkLookupByKeyMiss benchmarks a cache miss.
func BenchmarkLookupByKeyMiss(b *testing.B) {
	reg := &Registry{
		byKey: make(map[string]*Agent),
		byID:  make(map[string]*Agent),
	}
	b.ResetTimer()
	for range b.N {
		reg.LookupByKey("nonexistent")
	}
}
