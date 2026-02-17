package orchestrator

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractRequiredFeatures_Concurrency(t *testing.T) {
	// Ensures that the package-level regexes are safe for concurrent use
	const concurrency = 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	desc := `REQUIRED FEATURES:
- Feature 1: Description One
- Feature 2: Description Two
* Feature 3: Description Three`

	// Expected results:
	// "Feature 1: Description One" -> "feature 1: description one" -> "feature-1-description-one"
	// "Feature 2: Description Two" -> "feature 2: description two" -> "feature-2-description-two"
	// "Feature 3: Description Three" -> "feature 3: description three" -> "feature-3-description-three"

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			features := extractRequiredFeatures(desc)
			if assert.Len(t, features, 3) {
				// We expect IDs to start with "req-" followed by slug
				assert.Equal(t, "req-feature-1-description-one", features[0].ID)
				assert.Equal(t, "req-feature-2-description-two", features[1].ID)
				assert.Equal(t, "req-feature-3-description-three", features[2].ID)

				assert.Equal(t, "Feature 1: Description One", features[0].Description)
				assert.Equal(t, "Feature 2: Description Two", features[1].Description)
				assert.Equal(t, "Feature 3: Description Three", features[2].Description)
			}
		}()
	}
	wg.Wait()
}
