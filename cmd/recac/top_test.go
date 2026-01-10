package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTopCommandView(t *testing.T) {
	t.Run("displays headers correctly on initialization", func(t *testing.T) {
		sm := NewMockSessionManager()
		model := newTopModel(sm)

		view := model.View()

		assert.Contains(t, view, "NAME")
		assert.Contains(t, view, "STATUS")
		assert.Contains(t, view, "STARTED")
		assert.Contains(t, view, "DURATION")
		assert.Contains(t, view, "PROMPT")
		assert.Contains(t, view, "COMPLETION")
		assert.Contains(t, view, "TOTAL")
		assert.Contains(t, view, "COST")
	})

	t.Run("renders session data correctly", func(t *testing.T) {
		sm := NewMockSessionManager()
		// Add a running session
		_, err := sm.StartSession("running-session", []string{"sleep", "10"}, "/tmp")
		assert.NoError(t, err)

		// Add a completed session
		completedSession, err := sm.StartSession("completed-session", []string{"echo", "hello"}, "/tmp")
		assert.NoError(t, err)
		completedSession.Status = "completed"
		completedSession.EndTime = completedSession.StartTime.Add(10 * time.Second)
		sm.Sessions["completed-session"] = completedSession

		model := newTopModel(sm)
		model.Init() // Fetch initial data

		// Simulate an update
		sessions, _ := sm.ListSessions()
		msg := updateMsg(sessions)
		model.Update(msg)

		view := model.View()

		// Check for running session data
		assert.Contains(t, view, "running-session")
		assert.Contains(t, view, "running")

		// Check for completed session data
		assert.Contains(t, view, "completed-session")
		assert.Contains(t, view, "completed")
		assert.Contains(t, view, "10s") // Duration
	})
}

func TestTopCommandExists(t *testing.T) {
	rootCmd, _, _ := newRootCmd()
	cmd, _, err := rootCmd.Find([]string{"top"})
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
	assert.Equal(t, "top", cmd.Name())
}
