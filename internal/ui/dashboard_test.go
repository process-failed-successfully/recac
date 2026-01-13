package ui

import (
	"errors"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// MockSessionManager is a mock implementation of the ISessionManager interface for testing.
type MockSessionManager struct {
	sessions []*runner.SessionState
	err      error
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	return m.sessions, m.err
}

func TestDashboardModel_Update_sessionsRefreshedMsg(t *testing.T) {
	t.Run("successful refresh", func(t *testing.T) {
		// Arrange
		mockSessions := []*runner.SessionState{
			{Name: "test-session-1", Status: "running", StartTime: time.Now()},
			{Name: "test-session-2", Status: "completed", StartTime: time.Now().Add(-1 * time.Hour)},
		}
		mockSM := &MockSessionManager{sessions: mockSessions}
		model := NewDashboardModel(mockSM)

		// Act
		msg := sessionsRefreshedMsg{sessions: mockSessions}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(DashboardModel)

		// Assert
		require.False(t, m.loading, "loading should be false after refresh")
		require.Nil(t, m.err, "error should be nil")
		require.Len(t, m.table.Rows(), 2, "table should have 2 rows")
		require.Equal(t, "test-session-1", m.table.Rows()[0][0], "first session name is incorrect")
	})

	t.Run("refresh with error", func(t *testing.T) {
		// Arrange
		mockErr := errors.New("failed to list sessions")
		mockSM := &MockSessionManager{err: mockErr}
		model := NewDashboardModel(mockSM)

		// Act
		msg := sessionsRefreshedMsg{err: mockErr}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(DashboardModel)

		// Assert
		require.False(t, m.loading, "loading should be false after refresh")
		require.Error(t, m.err, "error should be set")
		require.Equal(t, "failed to list sessions", m.err.Error())
		require.Len(t, m.table.Rows(), 0, "table should be empty")
	})
}
