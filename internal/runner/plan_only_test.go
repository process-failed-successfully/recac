package runner

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/notify"

	"github.com/stretchr/testify/assert"
)

func TestRunLoop_PlanOnly(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()

	// Create app_spec.txt (Required)
	specPath := filepath.Join(tmpDir, "app_spec.txt")
	err := os.WriteFile(specPath, []byte("Implement a simple hello world"), 0644)
	assert.NoError(t, err)

	// Mock Agent
	mockAgent := agent.NewMockAgent()
	expectedPlan := "# Implementation Plan\n\n1. Do this.\n2. Do that."
	mockAgent.SetResponse(expectedPlan)

	// Create Session
	session := &Session{
		Workspace:     tmpDir,
		Agent:         mockAgent,
		Project:       "test-project",
		PlanOnly:      true,
		SpecFile:      "app_spec.txt",
		Logger:        slog.New(slog.NewTextHandler(os.Stdout, nil)),
		Notifier:      notify.NewManager(func(msg string, args ...interface{}) {}),
		SleepFunc:     func(d time.Duration) {},
		SlackThreadTS: "ts123", // To avoid empty check triggering start msg
	}

	// Run Loop
	ctx := context.Background()
	err = session.RunLoop(ctx)
	assert.NoError(t, err)

	// Assertions
	// 1. Check if PLAN.md exists
	planPath := filepath.Join(tmpDir, "PLAN.md")
	content, err := os.ReadFile(planPath)
	assert.NoError(t, err, "PLAN.md should exist")
	assert.Equal(t, expectedPlan, string(content), "PLAN.md content should match agent response")
}
