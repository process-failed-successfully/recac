package ui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// Mock SessionManager for testing
type mockSessionManager struct {
	sessions []*runner.SessionState
	err      error
}

func (m *mockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	return m.sessions, m.err
}

// All other ISessionManager methods are no-ops for this mock
func (m *mockSessionManager) SaveSession(*runner.SessionState) error { return nil }
func (m *mockSessionManager) LoadSession(name string) (*runner.SessionState, error) {
	return nil, nil
}
func (m *mockSessionManager) StopSession(name string) error              { return nil }
func (m *mockSessionManager) PauseSession(name string) error             { return nil }
func (m *mockSessionManager) ResumeSession(name string) error            { return nil }
func (m *mockSessionManager) GetSessionLogs(name string) (string, error) { return "", nil }
func (m *mockSessionManager) GetSessionLogContent(name string, lines int) (string, error) {
	return "", nil
}
func (m *mockSessionManager) StartSession(name, goal string, command []string, workspace string) (*runner.SessionState, error) {
	return nil, nil
}
func (m *mockSessionManager) GetSessionPath(name string) string           { return "" }
func (m *mockSessionManager) IsProcessRunning(pid int) bool               { return false }
func (m *mockSessionManager) RemoveSession(name string, force bool) error { return nil }
func (m *mockSessionManager) RenameSession(oldName, newName string) error { return nil }
func (m *mockSessionManager) SessionsDir() string                         { return "" }
func (m *mockSessionManager) GetSessionGitDiffStat(name string) (string, error) {
	return "", nil
}
func (m *mockSessionManager) ArchiveSession(name string) error   { return nil }
func (m *mockSessionManager) UnarchiveSession(name string) error { return nil }
func (m *mockSessionManager) ListArchivedSessions() ([]*runner.SessionState, error) {
	return nil, nil
}
func (m *mockSessionManager) RecoverSession(name string) (*runner.SessionState, error) {
	return nil, nil
}

func TestSummaryDashboard(t *testing.T) {
	// Mock agent.LoadState
	originalLoadState := agent.LoadState
	defer func() { agent.LoadState = originalLoadState }()

	// Mock runner.NewSessionManager
	originalNewSessionManager := runner.NewSessionManager
	defer func() { runner.NewSessionManager = originalNewSessionManager }()

	t.Run("Init", func(t *testing.T) {
		model := NewSummaryModel()
		cmd := model.Init()
		assert.NotNil(t, cmd)

		// The command returned by Init should produce a BatchMsg when executed
		msg := cmd()
		_, ok := msg.(tea.BatchMsg)
		assert.True(t, ok, "Init command should produce a tea.BatchMsg")
	})

	t.Run("Update", func(t *testing.T) {
		model := NewSummaryModel()

		testCases := []struct {
			name     string
			msg      tea.Msg
			testFunc func(t *testing.T, model tea.Model, cmd tea.Cmd)
		}{
			{
				name: "Quit on 'q'",
				msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}},
				testFunc: func(t *testing.T, model tea.Model, cmd tea.Cmd) {
					assert.Equal(t, reflect.ValueOf(tea.Quit).Pointer(), reflect.ValueOf(cmd).Pointer())
				},
			},
			{
				name: "Quit on 'ctrl+c'",
				msg:  tea.KeyMsg{Type: tea.KeyCtrlC},
				testFunc: func(t *testing.T, model tea.Model, cmd tea.Cmd) {
					assert.Equal(t, reflect.ValueOf(tea.Quit).Pointer(), reflect.ValueOf(cmd).Pointer())
				},
			},
			{
				name: "Refresh on tick",
				msg:  summaryTickMsg(time.Now()),
				testFunc: func(t *testing.T, model tea.Model, cmd tea.Cmd) {
					assert.NotNil(t, cmd)
					// Further inspection could be done here if needed
				},
			},
			{
				name: "Update sessions on refresh message",
				msg: sessionsRefreshedMsg{
					{Name: "test-session", Status: "running"},
				},
				testFunc: func(t *testing.T, model tea.Model, cmd tea.Cmd) {
					m := model.(summaryModel)
					assert.Len(t, m.sessions, 1)
					assert.Equal(t, "test-session", m.sessions[0].Name)
					assert.Nil(t, cmd)
				},
			},
			{
				name: "Handle error message",
				msg:  errors.New("test error"),
				testFunc: func(t *testing.T, model tea.Model, cmd tea.Cmd) {
					m := model.(summaryModel)
					assert.EqualError(t, m.err, "test error")
					assert.Nil(t, cmd)
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				m, cmd := model.Update(tc.msg)
				tc.testFunc(t, m, cmd)
			})
		}
	})

	t.Run("View", func(t *testing.T) {
		tempDir := t.TempDir()
		stateFile := filepath.Join(tempDir, "agent_state.json")
		err := os.WriteFile(stateFile, []byte(`{"model": "test-model", "tokenUsage": {"totalTokens": 100}}`), 0644)
		assert.NoError(t, err)

		agent.LoadState = func(path string) (*agent.State, error) {
			if path == stateFile {
				return &agent.State{
					Model:      "test-model",
					TokenUsage: agent.TokenUsage{TotalTokens: 100},
				}, nil
			}
			return nil, errors.New("file not found")
		}

		t.Run("Error view", func(t *testing.T) {
			model := NewSummaryModel()
			model.err = errors.New("test error")
			view := model.View()
			assert.Contains(t, view, "Error: test error")
		})

		t.Run("No sessions view", func(t *testing.T) {
			model := NewSummaryModel()
			model.lastUpdate = time.Now()
			view := model.View()
			assert.Contains(t, view, "No sessions found")
		})

		t.Run("With sessions view", func(t *testing.T) {
			model := NewSummaryModel()
			model.lastUpdate = time.Now()
			model.sessions = []*runner.SessionState{
				{Name: "session-1", Status: "completed", StartTime: time.Now().Add(-1 * time.Hour), EndTime: time.Now(), AgentStateFile: stateFile},
				{Name: "session-2", Status: "running", StartTime: time.Now().Add(-30 * time.Minute)},
				{Name: "session-3", Status: "error", StartTime: time.Now().Add(-2 * time.Hour)},
			}

			view := model.View()

			// Use regex for flexible spacing to avoid tabwriter issues
			assert.Regexp(t, regexp.MustCompile(`Total Sessions:\s+3`), view)
			assert.Regexp(t, regexp.MustCompile(`Completed:\s+1`), view)
			assert.Regexp(t, regexp.MustCompile(`Errored:\s+1`), view)
			assert.Regexp(t, regexp.MustCompile(`Running:\s+1`), view)
			assert.Regexp(t, regexp.MustCompile(`Success Rate:\s+33.33%`), view)

			assert.Contains(t, view, "Recent Sessions (Top 5)")
			assert.True(t, strings.Contains(view, "session-1") && strings.Contains(view, "session-2") && strings.Contains(view, "session-3"))

			assert.Contains(t, view, "Most Expensive Sessions (Top 5)")
			assert.Regexp(t, regexp.MustCompile(`session-1\s+\$0.0001`), view)
			assert.Regexp(t, regexp.MustCompile(`\s+100\s+test-model`), view)
		})
	})

	t.Run("refreshSessionsCmd", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			runner.NewSessionManager = func() (runner.ISessionManager, error) {
				return &mockSessionManager{
					sessions: []*runner.SessionState{{Name: "test"}},
				}, nil
			}

			cmd := refreshSessionsCmd()
			msg := cmd()
			refreshedMsg, ok := msg.(sessionsRefreshedMsg)
			assert.True(t, ok)
			assert.Len(t, refreshedMsg, 1)
			assert.Equal(t, "test", refreshedMsg[0].Name)
		})

		t.Run("NewSessionManager fails", func(t *testing.T) {
			runner.NewSessionManager = func() (runner.ISessionManager, error) {
				return nil, errors.New("new manager error")
			}

			cmd := refreshSessionsCmd()
			msg := cmd()
			err, ok := msg.(error)
			assert.True(t, ok)
			assert.EqualError(t, err, "new manager error")
		})

		t.Run("ListSessions fails", func(t *testing.T) {
			runner.NewSessionManager = func() (runner.ISessionManager, error) {
				return &mockSessionManager{
					err: errors.New("list error"),
				}, nil
			}

			cmd := refreshSessionsCmd()
			msg := cmd()
			err, ok := msg.(error)
			assert.True(t, ok)
			assert.EqualError(t, err, "list error")
		})
	})
}
