package ui

import (
	"fmt"
	"recac/internal/model"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestTopDashboardModel_Init(t *testing.T) {
	m := NewTopDashboardModel()
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestTopDashboardModel_Update(t *testing.T) {
	m := NewTopDashboardModel()

	t.Run("quit message", func(t *testing.T) {
		newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		assert.Equal(t, m, newM)
		assert.NotNil(t, cmd)
	})

	t.Run("window size", func(t *testing.T) {
		newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
		updatedM := newM.(topDashboardModel)
		assert.Equal(t, 100, updatedM.width)
		assert.Equal(t, 50, updatedM.height)
	})

	t.Run("successful session fetch", func(t *testing.T) {
		sessions := []model.UnifiedSession{
			{Name: "s1", Status: "running", StartTime: time.Now(), Goal: "test goal"},
		}
		newM, _ := m.Update(topSessionsRefreshedMsg(sessions))
		updatedM := newM.(topDashboardModel)
		assert.Len(t, updatedM.sessions, 1)
		assert.Len(t, updatedM.table.Rows(), 1)
	})

	t.Run("error message", func(t *testing.T) {
		err := fmt.Errorf("fetch error")
		newM, _ := m.Update(err)
		updatedM := newM.(topDashboardModel)
		assert.Equal(t, err, updatedM.err)
	})
}

func TestTopDashboardModel_View(t *testing.T) {
	m := NewTopDashboardModel()

	// Error view
	m.err = fmt.Errorf("some error")
	assert.Contains(t, m.View(), "Error: some error")

	// Normal view
	m.err = nil
	assert.Contains(t, m.View(), "RECAC Top Dashboard")
}

func TestRefreshTopSessionsCmd(t *testing.T) {
	originalGetTopSessions := GetTopSessions
	defer func() { GetTopSessions = originalGetTopSessions }()

	t.Run("success", func(t *testing.T) {
		GetTopSessions = func() ([]model.UnifiedSession, error) {
			return []model.UnifiedSession{}, nil
		}
		cmd := refreshTopSessionsCmd()
		msg := cmd()
		_, ok := msg.(topSessionsRefreshedMsg)
		assert.True(t, ok)
	})

	t.Run("error", func(t *testing.T) {
		GetTopSessions = func() ([]model.UnifiedSession, error) {
			return nil, fmt.Errorf("failed")
		}
		cmd := refreshTopSessionsCmd()
		msg := cmd()
		err, ok := msg.(error)
		assert.True(t, ok)
		assert.EqualError(t, err, "failed")
	})

	t.Run("nil GetTopSessions", func(t *testing.T) {
		GetTopSessions = nil
		cmd := refreshTopSessionsCmd()
		msg := cmd()
		err, ok := msg.(error)
		assert.True(t, ok)
		assert.Contains(t, err.Error(), "GetTopSessions function is not set")
	})
}
