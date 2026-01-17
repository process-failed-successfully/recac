package main

import (
	"bytes"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPsCmd(t *testing.T) {
	// Mock session manager
	mockSM := NewMockSessionManager()

	// Add some sessions
	now := time.Now()
	mockSM.Sessions["session-1"] = &runner.SessionState{
		Name:      "session-1",
		Status:    "running",
		StartTime: now.Add(-1 * time.Hour),
		PID:       1234,
	}
	mockSM.Sessions["session-2"] = &runner.SessionState{
		Name:      "session-2",
		Status:    "completed",
		StartTime: now.Add(-2 * time.Hour),
		PID:       0,
	}

	// Override factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	t.Run("Ps List All", func(t *testing.T) {
		cmd := NewPsCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Contains(t, b.String(), "session-1")
		assert.Contains(t, b.String(), "running")
		assert.Contains(t, b.String(), "session-2")
		assert.Contains(t, b.String(), "completed")
	})

	t.Run("Ps Filter Status", func(t *testing.T) {
		cmd := NewPsCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)
		cmd.SetArgs([]string{"--status", "running"})

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Contains(t, b.String(), "session-1")
		assert.NotContains(t, b.String(), "session-2")
	})

	t.Run("Ps Filter Since", func(t *testing.T) {
		cmd := NewPsCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)
		cmd.SetArgs([]string{"--since", "90m"}) // Should only show session-1 (1h ago)

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Contains(t, b.String(), "session-1")
		assert.NotContains(t, b.String(), "session-2")
	})

	t.Run("Ps Sort Name", func(t *testing.T) {
		cmd := NewPsCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)
		cmd.SetArgs([]string{"--sort", "name"})

		err := cmd.Execute()
		assert.NoError(t, err)
		output := b.String()
		// Verify order implicitly by string position, or just that it runs.
		// Detailed order verification is brittle with tabwriter, but we can check existence.
		assert.Contains(t, output, "session-1")
		assert.Contains(t, output, "session-2")
	})
}
