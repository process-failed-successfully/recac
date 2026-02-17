package orchestrator

import (
	"sync"
	"testing"
)

func TestExtractRequiredFeatures_Concurrency(t *testing.T) {
	// Simulates concurrent access to ensure thread safety of the regexes
	concurrency := 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	input := `REQUIRED FEATURES:
- Feature 1
- Feature 2`

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			_ = extractRequiredFeatures(input)
		}()
	}

	wg.Wait()
}
