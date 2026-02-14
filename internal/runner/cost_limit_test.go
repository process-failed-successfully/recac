package runner

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunLoop_MaxCostLimit(t *testing.T) {
	// 1. Setup Temp Workspace
	workspace, err := os.MkdirTemp("", "recac-cost-test")
	assert.NoError(t, err)
	defer os.RemoveAll(workspace)

	// Create app_spec.txt (Required by RunLoop)
	err = os.WriteFile(filepath.Join(workspace, "app_spec.txt"), []byte("Test Spec"), 0644)
	assert.NoError(t, err)

	// 2. Setup Mock Dependencies
	mockAgent := agent.NewMockAgent()
	// Set a valid response to avoid NoOp errors
	mockAgent.SetResponse("I am working on the task. Here is a command:\n```bash\necho 'Hello'\n```")

	// 3. Initialize Session with Low MaxCost
	// Use NewSession but pass nil for Docker (restricted mode)
	session := NewSession(nil, mockAgent, workspace, "test-image", "test-project", "mock-provider", "gpt-4o", 1)
	session.MaxCost = 0.000001 // Extremely low limit ($0.000001)

	// Mock Sleep to speed up test
	session.SleepFunc = func(d time.Duration) {}

	// 4. Manually Inject High Token Usage into State
	// We need to write a state file that implies high cost.
	// GPT-4o input cost is $5.00 / 1M tokens = $0.000005 / token.
	// 1000 tokens = $0.005 > $0.000001.
	highUsageState := agent.State{
		Model: "gpt-4o",
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:   1000,
			TotalResponseTokens: 0,
			TotalTokens:         1000,
		},
	}
	// We need to ensure state manager is initialized. NewSession does it.
	err = session.StateManager.Save(highUsageState)
	assert.NoError(t, err)

	// 5. Run Loop
	// RunLoop should check cost immediately (or after first iteration depending on logic) and fail.

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = session.RunLoop(ctx)

	// 6. Assert Error
	assert.ErrorIs(t, err, ErrMaxCost)
}
