package ui

import (
	"errors"
	"recac/internal/model"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestPsDashboardModel_Init(t *testing.T) {
	m := NewPsDashboardModel()
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestPsDashboardModel_Update(t *testing.T) {
	// Keep a consistent time for testing "LAST USED"
	testTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	testCases := []struct {
		name      string
		msg       tea.Msg
		mockSetup func()
		verify    func(t *testing.T, m tea.Model, cmd tea.Cmd)
	}{
		{
			name: "quit message",
			msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")},
			verify: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				// Direct function comparison is not reliable, so we compare their pointers.
				assert.Equal(t, reflect.ValueOf(tea.Quit).Pointer(), reflect.ValueOf(cmd).Pointer())
			},
		},
		{
			name: "successful session fetch",
			msg: psSessionsRefreshedMsg{
				{Name: "test-session", Status: "Running", Goal: "Test the dashboard", LastActivity: testTime},
			},
			verify: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model, ok := m.(psDashboardModel)
				assert.True(t, ok)
				assert.Len(t, model.sessions, 1)
				assert.Equal(t, "test-session", model.sessions[0].Name)
				assert.Nil(t, cmd)
			},
		},
		{
			name: "error message",
			msg:  errors.New("test error"),
			verify: func(t *testing.T, m tea.Model, cmd tea.Cmd) {
				model, ok := m.(psDashboardModel)
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

			m := NewPsDashboardModel()
			updatedModel, cmd := m.Update(tc.msg)
			tc.verify(t, updatedModel, cmd)
		})
	}
}

func TestPsDashboardModel_View(t *testing.T) {
	testTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	m := NewPsDashboardModel()
	// Set a width to avoid unexpected truncation by the table component.
	// The component would normally receive this from a tea.WindowSizeMsg.
	m.table.SetWidth(200)

	m.sessions = []model.UnifiedSession{
		{Name: "session-1", Status: "Running", Goal: "A very long goal that should be truncated here in the test case", LastActivity: testTime, Location: "local"},
		{Name: "session-2", Status: "Stopped", Goal: "Short goal", LastActivity: testTime.Add(-24 * time.Hour), Location: "k8s", StartTime: testTime.Add(-24 * time.Hour)},
	}
	m.lastUpdate = testTime
	m.updateTableRows()

	view := m.View()

	assert.Contains(t, view, "RECAC PS Dashboard")
	assert.Contains(t, view, "session-1")
	assert.Contains(t, view, "Running")
	// Make the assertion less brittle, as the table renderer adds spaces.
	// The core truncation logic is tested in TestPsDashboardModel_UpdateTableRows.
	assert.Contains(t, view, "A very long goal that should be truncated")
	assert.Contains(t, view, "session-2")
	assert.Contains(t, view, "Stopped")
	assert.Contains(t, view, "Short goal")

	m.err = errors.New("render error")
	view = m.View()
	assert.Contains(t, view, "Error: render error")
}

func TestRefreshPsSessionsCmd(t *testing.T) {
	defer func() { GetSessions = nil }()

	t.Run("success", func(t *testing.T) {
		GetSessions = func() ([]model.UnifiedSession, error) {
			return []model.UnifiedSession{{Name: "test"}}, nil
		}

		cmd := refreshPsSessionsCmd()
		msg := cmd()
		sessions, ok := msg.(psSessionsRefreshedMsg)
		assert.True(t, ok)
		assert.Len(t, sessions, 1)
		assert.Equal(t, "test", sessions[0].Name)
	})

	t.Run("error", func(t *testing.T) {
		GetSessions = func() ([]model.UnifiedSession, error) {
			return nil, errors.New("fetch error")
		}

		cmd := refreshPsSessionsCmd()
		msg := cmd()
		err, ok := msg.(error)
		assert.True(t, ok)
		assert.Error(t, err)
		assert.Equal(t, "fetch error", err.Error())
	})

	t.Run("nil GetSessions", func(t *testing.T) {
		GetSessions = nil
		cmd := refreshPsSessionsCmd()
		msg := cmd()
		err, ok := msg.(error)
		assert.True(t, ok)
		assert.Error(t, err)
		assert.Equal(t, "GetSessions function is not set", err.Error())
	})
}

func TestStartPsDashboard_Error(t *testing.T) {
	// This is a placeholder; in a real test, you might need a more robust way
	// to inject this error, as bubbletea's Program doesn't expose Run errors easily.
	t.Skip("Skipping because intercepting bubbletea's Run method is non-trivial in a unit test")
}

func TestPsDashboardModel_UpdateTableRows(t *testing.T) {
	now := time.Now()
	longGoal := "This is a very long goal that is definitely going to be truncated"
	m := NewPsDashboardModel()
	m.sessions = []model.UnifiedSession{
		{Name: "local-session", Status: "Running", Goal: "Local test", LastActivity: now, Location: "local", CPU: "10%", Memory: "100MB"},
		{Name: "k8s-session", Status: "Running", Goal: "K8s test", StartTime: now.Add(-10 * time.Minute), Location: "k8s"},
		{Name: "long-goal-session", Status: "Running", Goal: longGoal, LastActivity: now, Location: "local"},
	}

	m.updateTableRows()

	rows := m.table.Rows()
	assert.Len(t, rows, 3)
	// Row 0: local-session
	assert.Equal(t, "local-session", rows[0][0])
	assert.Equal(t, "10%", rows[0][2])   // CPU
	assert.Equal(t, "100MB", rows[0][3]) // Memory
	assert.Equal(t, "local", rows[0][4]) // Location
	assert.True(t, strings.Contains(rows[0][5], "ago"))

	// Row 1: k8s-session
	assert.Equal(t, "k8s-session", rows[1][0])
	assert.Equal(t, "", rows[1][2])     // CPU (empty)
	assert.Equal(t, "", rows[1][3])     // Memory (empty)
	assert.Equal(t, "k8s", rows[1][4])  // Location
	assert.Equal(t, "10m ago", rows[1][5])

	// Row 2: long-goal-session (truncation check)
	assert.Equal(t, "This is a very long goal that is definitely going to b...", rows[2][6])
}
