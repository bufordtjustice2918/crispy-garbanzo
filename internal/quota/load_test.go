package quota

import (
	"sync"
	"testing"
)

// TestConcurrentCheck hammers the limiter from 100 goroutines.
func TestConcurrentCheck(t *testing.T) {
	lim, _ := NewLimiter("/nonexistent")
	lim.Set(Limit{AgentID: "a1", RPS: 1000000, Mode: "hard_stop"})

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				d := lim.Check("a1")
				_ = d
			}
		}()
	}
	wg.Wait()
}

// TestConcurrentSetCheck tests concurrent Set/Remove/Check.
func TestConcurrentSetCheck(t *testing.T) {
	lim, _ := NewLimiter("/nonexistent")

	var wg sync.WaitGroup

	// 50 checkers.
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 500 {
				lim.Check("a1")
				lim.Check("unknown")
				lim.All()
			}
		}()
	}

	// 10 writers.
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				lim.Set(Limit{AgentID: "a1", RPS: 1000, Mode: "hard_stop"})
				lim.Remove("a1")
			}
		}()
	}

	wg.Wait()
}
