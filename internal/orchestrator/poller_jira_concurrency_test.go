package orchestrator

import (
	"sync"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestExtractRequiredFeatures_Concurrency(t *testing.T) {
	// Sample input text
	text := `
		Some description.
		REQUIRED FEATURES:
		- Feature A
		- Feature B
	`

	var wg sync.WaitGroup
	concurrency := 100

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			features := extractRequiredFeatures(text)
			assert.Len(t, features, 2)
			assert.Equal(t, "Feature A", features[0].Description)
			assert.Equal(t, "Feature B", features[1].Description)
		}()
	}

	wg.Wait()
}
