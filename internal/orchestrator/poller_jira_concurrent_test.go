package orchestrator

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestJiraPoller_Poll_Concurrent(t *testing.T) {
	mockClient := new(MockJiraClient)
	// Expect multiple concurrent calls. testify/mock is thread-safe.
	mockClient.On("SearchIssues", mock.Anything, DefaultJQL).Return([]map[string]interface{}{}, nil)

	poller := NewJiraPoller(mockClient, "") // Empty JQL triggers default logic

	var wg sync.WaitGroup
	count := 100
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			_, err := poller.Poll(context.Background(), nil)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
}

func TestExtractRequiredFeatures_Concurrent(t *testing.T) {
	text := `
Some description here.

REQUIRED FEATURES:
- Feature One
- Feature Two
`
	var wg sync.WaitGroup
	// Run 100 concurrent goroutines
	concurrency := 100
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			features := extractRequiredFeatures(text)

			// Basic checks
			assert.Len(t, features, 2)

			// Check feature extraction logic
			// Note: We avoid checking internal state/memory of regex directly,
			// but verify the output is correct and consistent.
			if len(features) > 0 {
				assert.Equal(t, "req-feature-one", features[0].ID)
			}
			if len(features) > 1 {
				assert.Equal(t, "req-feature-two", features[1].ID)
			}
		}()
	}
	wg.Wait()
}
