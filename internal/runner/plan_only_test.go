package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/notify"
	"recac/internal/telemetry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanOnlyMode(t *testing.T) {
	// 1. Setup Workspace
	workspace := t.TempDir()
	specContent := "Build a crypto bot"
	err := os.WriteFile(filepath.Join(workspace, "app_spec.txt"), []byte(specContent), 0644)
	require.NoError(t, err)

	// 2. Setup Mock Agent
	mockAgent := agent.NewMockAgent()
	expectedPlan := "# Implementation Plan\n\n1. Step One\n2. Step Two"
	mockAgent.SetResponse(expectedPlan)

	// 3. Setup Session
	// Use No-Op logger
	logger := telemetry.NewLogger(false, "", false)

	session := &Session{
		Workspace:     workspace,
		Agent:         mockAgent,
		PlanOnly:      true,
		Logger:        logger,
		Project:       "test-project",
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		SleepFunc:     func(d time.Duration) {},
		SpecFile:      "app_spec.txt",
	}

	// 4. Run Loop
	ctx := context.Background()
	err = session.RunLoop(ctx)
	require.NoError(t, err)

	// 5. Verify PLAN.md
	planPath := filepath.Join(workspace, "PLAN.md")
	assert.FileExists(t, planPath)

	content, err := os.ReadFile(planPath)
	require.NoError(t, err)
	assert.Equal(t, expectedPlan, string(content))
}
