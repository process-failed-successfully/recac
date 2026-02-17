package orchestrator

import (
	"sync"
	"testing"
)

func TestExtractRequiredFeatures_Concurrent(t *testing.T) {
	// This test ensures that the package-level regexes work correctly under concurrent access.

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	input := `
REQUIRED FEATURES:
- Feature A
- Feature B
`

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			features := extractRequiredFeatures(input)
			if len(features) != 2 {
				t.Errorf("expected 2 features, got %d", len(features))
			}
		}()
	}

	wg.Wait()
}
