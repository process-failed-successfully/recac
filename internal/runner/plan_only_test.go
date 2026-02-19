package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/telemetry"
	"github.com/stretchr/testify/assert"
)

func TestRunPlanOnly(t *testing.T) {
	// Setup Workspace
	workspace, err := os.MkdirTemp("", "recac-test-plan-*")
	assert.NoError(t, err)
	defer os.RemoveAll(workspace)

	// Create app_spec.txt
	err = os.WriteFile(filepath.Join(workspace, "app_spec.txt"), []byte("Test Spec"), 0644)
	assert.NoError(t, err)

	// Setup Mock Agent
	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("# Test Plan\n\n1. Do this.\n2. Do that.")

	// Create Session
	logger := telemetry.NewLogger(false, "", true)
	session := &Session{
		Workspace: workspace,
		Agent:     mockAgent,
		Logger:    logger,
		SpecFile:  "app_spec.txt",
		PlanOnly:  true,
		SleepFunc: func(d time.Duration) {},
	}

	// Execute
	err = session.RunPlanOnly(context.Background())
	assert.NoError(t, err)

	// Verify PLAN.md
	planPath := filepath.Join(workspace, "PLAN.md")
	content, err := os.ReadFile(planPath)
	assert.NoError(t, err)
	assert.Equal(t, "# Test Plan\n\n1. Do this.\n2. Do that.", string(content))
}
