package ui

import (
	"errors"
	"recac/internal/model"
	"reflect"
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
	testTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	testCases := []struct {
		name      string
		msg       tea.Msg
		mockSetup func()
		verify    func(t *testing.T, m tea.Model, cmd tea.Cmd)
	}{
		{
			name: "window size",
			msg:  tea.WindowSizeMsg{Width: 100, Height: 50},
			verify: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model, ok := m.(topDashboardModel)
				assert.True(t, ok)
				assert.Equal(t, 100, model.width)
				assert.Equal(t, 50, model.height)
				assert.Nil(t, cmd)
			},
		},
		{
			name: "quit key q",
			msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")},
			verify: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				assert.Equal(t, reflect.ValueOf(tea.Quit).Pointer(), reflect.ValueOf(cmd).Pointer())
			},
		},
		{
			name: "quit key ctrl+c",
			msg:  tea.KeyMsg{Type: tea.KeyCtrlC},
			verify: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				assert.Equal(t, reflect.ValueOf(tea.Quit).Pointer(), reflect.ValueOf(cmd).Pointer())
			},
		},
		{
			name: "tick message",
			msg:  topTickMsg(testTime),
			verify: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				// Should return a command to refresh sessions
				assert.NotNil(t, cmd)
			},
		},
		{
			name: "sessions refreshed",
			msg: topSessionsRefreshedMsg{
				{Name: "session-1", Status: "Running", CPU: "10%", Memory: "100MB", StartTime: testTime, Goal: "Test goal"},
			},
			verify: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model, ok := m.(topDashboardModel)
				assert.True(t, ok)
				assert.Len(t, model.sessions, 1)
				assert.Equal(t, "session-1", model.sessions[0].Name)
				assert.Nil(t, cmd)

				rows := model.table.Rows()
				assert.Len(t, rows, 1)
				assert.Equal(t, "session-1", rows[0][0])
			},
		},
		{
			name: "error",
			msg:  errors.New("test error"),
			verify: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model, ok := m.(topDashboardModel)
				assert.True(t, ok)
				assert.Error(t, model.err)
				assert.Equal(t, "test error", model.err.Error())
				assert.Nil(t, cmd)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.mockSetup != nil {
				tc.mockSetup()
			}
			m := NewTopDashboardModel()
			updatedModel, cmd := m.Update(tc.msg)
			tc.verify(t, updatedModel, cmd)
		})
	}
}

func TestTopDashboardModel_View(t *testing.T) {
	testTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	m := NewTopDashboardModel()
	m.table.SetWidth(200)

	m.sessions = []model.UnifiedSession{
		{Name: "session-1", Status: "Running", CPU: "10%", Memory: "100MB", StartTime: testTime, Goal: "Test goal"},
	}
	m.lastUpdate = testTime
	m.updateTableRows()

	view := m.View()

	assert.Contains(t, view, "RECAC Top Dashboard")
	assert.Contains(t, view, "session-1")
	assert.Contains(t, view, "Running")
	assert.Contains(t, view, "Test goal")

	m.err = errors.New("render error")
	view = m.View()
	assert.Contains(t, view, "Error: render error")
}

func TestRefreshTopSessionsCmd(t *testing.T) {
	// Restore original function after test
	original := GetTopSessions
	defer func() { GetTopSessions = original }()

	t.Run("success", func(t *testing.T) {
		GetTopSessions = func() ([]model.UnifiedSession, error) {
			return []model.UnifiedSession{{Name: "test"}}, nil
		}

		cmd := refreshTopSessionsCmd()
		msg := cmd()
		sessions, ok := msg.(topSessionsRefreshedMsg)
		assert.True(t, ok)
		assert.Len(t, sessions, 1)
		assert.Equal(t, "test", sessions[0].Name)
	})

	t.Run("error", func(t *testing.T) {
		GetTopSessions = func() ([]model.UnifiedSession, error) {
			return nil, errors.New("fetch error")
		}

		cmd := refreshTopSessionsCmd()
		msg := cmd()
		err, ok := msg.(error)
		assert.True(t, ok)
		assert.Error(t, err)
		assert.Equal(t, "fetch error", err.Error())
	})

	t.Run("nil GetTopSessions", func(t *testing.T) {
		GetTopSessions = nil
		cmd := refreshTopSessionsCmd()
		msg := cmd()
		err, ok := msg.(error)
		assert.True(t, ok)
		assert.Error(t, err)
		assert.Equal(t, "GetTopSessions function is not set", err.Error())
	})
}

func TestStartTopDashboard_Error(t *testing.T) {
	t.Skip("Skipping because intercepting bubbletea's Run method is non-trivial in a unit test")
}
