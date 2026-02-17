package orchestrator

import (
	"fmt"
	"sync"
	"testing"
)

// TestExtractRequiredFeatures_Concurrency tests the thread-safety of
// regex usage in extractRequiredFeatures.
func TestExtractRequiredFeatures_Concurrency(t *testing.T) {
	// If regex usage is not thread-safe, this test run with -race should detect it, or it might panic.

	var wg sync.WaitGroup
	numRoutines := 100

	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Construct a text that triggers both regexes
			desc := fmt.Sprintf("REQUIRED FEATURES:\n- Feature %d", id)

			features := extractRequiredFeatures(desc)

			if len(features) != 1 {
				t.Errorf("Expected 1 feature for routine %d, got %d", id, len(features))
			}

			expectedSlug := fmt.Sprintf("req-feature-%d", id)
			if features[0].ID != expectedSlug {
				t.Errorf("Expected slug %s, got %s", expectedSlug, features[0].ID)
			}
		}(i)
	}
	wg.Wait()
}
