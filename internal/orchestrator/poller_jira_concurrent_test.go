package orchestrator

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractRequiredFeatures_Concurrent verifies that the package-level regexes
// used in extractRequiredFeatures are thread-safe.
func TestExtractRequiredFeatures_Concurrent(t *testing.T) {
	text := `
REQUIRED FEATURES:
- Feature A
* Feature B
- Feature C
`
	var wg sync.WaitGroup
	// High concurrency to increase chance of race detection
	concurrency := 100

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			features := extractRequiredFeatures(text)
			assert.Len(t, features, 3)
			if len(features) > 0 {
				assert.Equal(t, "Feature A", features[0].Description)
				assert.Equal(t, "req-feature-a", features[0].ID)
			}
		}()
	}
	wg.Wait()
}

// TestJiraPoller_Poll_Concurrent verifies that Poll method is thread-safe
// regarding regex usage and internal state.
func TestJiraPoller_Poll_Concurrent(t *testing.T) {
	// Mock Jira Client
	mockClient := new(MockJiraClient)
	poller := NewJiraPoller(mockClient, "status = 'To Do'")

	ctx := context.Background()

	// Setup mock behavior
	desc := "Repo: https://github.com/test/repo\nREQUIRED FEATURES:\n- Feature A"
	issue := mockIssue("PROJ-FEAT", "Feature Request", desc)

	concurrency := 50

	// Expect calls 'concurrency' times
	mockClient.On("SearchIssues", ctx, "status = 'To Do'").Return([]map[string]interface{}{issue}, nil).Times(concurrency)
	// GetBlockerKeys is called twice per Poll (once in BuildGraphFromIssues, once in filtering)
	mockClient.On("GetBlockerKeys", issue).Return([]string{}).Times(concurrency * 2)
	mockClient.On("ParseDescription", issue).Return(desc).Times(concurrency)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Poll
			items, err := poller.Poll(ctx, nil)
			assert.NoError(t, err)
			if len(items) > 0 {
				assert.Equal(t, "PROJ-FEAT", items[0].ID)
			}
		}()
	}
	wg.Wait()
}
