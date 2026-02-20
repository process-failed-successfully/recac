package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"
	"recac/internal/telemetry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession_GeneratePlanOnly(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "app_spec.txt")
	err := os.WriteFile(specFile, []byte("Test Spec"), 0644)
	require.NoError(t, err)

	mockAgent := agent.NewMockAgent()
	expectedPlan := "# Implementation Plan\n\n1. Do this.\n2. Do that."
	mockAgent.SetResponse(expectedPlan)

	// Create a minimal session
	session := &Session{
		Workspace:    tmpDir,
		SpecFile:     "app_spec.txt",
		Agent:        mockAgent,
		Logger:       telemetry.NewLogger(false, "", true),
		StreamOutput: false,
	}

	// Execute
	err = session.GeneratePlanOnly(context.Background())

	// Verify
	require.NoError(t, err)

	planPath := filepath.Join(tmpDir, "PLAN.md")
	content, err := os.ReadFile(planPath)
	require.NoError(t, err)
	assert.Equal(t, expectedPlan, string(content))
}
