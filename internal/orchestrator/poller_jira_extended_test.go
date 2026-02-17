package orchestrator

import (
	"fmt"
	"sync"
	"testing"
)

// TestExtractRequiredFeatures_Concurrency tests the thread-safe lazy initialization
// of regexes in extractRequiredFeatures.
func TestExtractRequiredFeatures_Concurrency(t *testing.T) {
	// If initialization is not thread-safe (e.g. race condition on assignment),
	// this test run with -race should detect it, or it might panic.

	// Reset the Once (not possible via public API, so we assume fresh state or rely on idempotency)
	// Actually, we can't reset sync.Once. But we can ensure that calling it concurrently doesn't crash.

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
