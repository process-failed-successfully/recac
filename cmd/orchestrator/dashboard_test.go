package main

import (
	"context"
	"testing"
	"time"

	"recac/internal/runner"
	"recac/internal/ui"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSessionManager
type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*runner.SessionState), args.Error(1)
}

func (m *MockSessionManager) SaveSession(s *runner.SessionState) error { return nil }
func (m *MockSessionManager) LoadSession(name string) (*runner.SessionState, error) { return nil, nil }
func (m *MockSessionManager) StopSession(name string) error { return nil }
func (m *MockSessionManager) PauseSession(name string) error { return nil }
func (m *MockSessionManager) ResumeSession(name string) error { return nil }
func (m *MockSessionManager) GetSessionLogs(name string) (string, error) { return "", nil }
func (m *MockSessionManager) GetSessionLogContent(name string, lines int) (string, error) { return "", nil }
func (m *MockSessionManager) StartSession(name, goal string, command []string, workspace string) (*runner.SessionState, error) { return nil, nil }
func (m *MockSessionManager) GetSessionPath(name string) string { return "" }
func (m *MockSessionManager) IsProcessRunning(pid int) bool { return false }
func (m *MockSessionManager) RemoveSession(name string, force bool) error { return nil }
func (m *MockSessionManager) RenameSession(oldName, newName string) error { return nil }
func (m *MockSessionManager) SessionsDir() string { return "" }
func (m *MockSessionManager) GetSessionGitDiffStat(name string) (string, error) { return "", nil }
func (m *MockSessionManager) ArchiveSession(name string) error { return nil }
func (m *MockSessionManager) UnarchiveSession(name string) error { return nil }
func (m *MockSessionManager) ListArchivedSessions() ([]*runner.SessionState, error) { return nil, nil }

func TestDashboardCmd_Flags(t *testing.T) {
	// Test flags parsing manually to avoid side effects
	showCosts, _ := dashboardCmd.Flags().GetBool("show-costs")
	assert.False(t, showCosts)

	// We can't easily test parsing via SetArgs without Execute, which runs the command.
	// But we test it in TestRunDashboard_Integration
}

func TestFetchLocalSessions(t *testing.T) {
	// Mock SessionManager
	mockSM := new(MockSessionManager)
	originalNewSM := runner.NewSessionManager
	defer func() { runner.NewSessionManager = originalNewSM }()

	runner.NewSessionManager = func() (runner.ISessionManager, error) {
		return mockSM, nil
	}

	// Setup expectations
	now := time.Now()
	sessions := []*runner.SessionState{
		{Name: "test-session", Status: "running", StartTime: now},
	}
	mockSM.On("ListSessions").Return(sessions, nil)

	// Run fetch
	unified, err := fetchLocalSessions(context.Background())
	assert.NoError(t, err)
	assert.Len(t, unified, 1)
	assert.Equal(t, "test-session", unified[0].Name)
	assert.Equal(t, "running", unified[0].Status)
	assert.Equal(t, "local", unified[0].Location)
}

func TestRunDashboard_Integration(t *testing.T) {
	// Mock StartPsDashboard
	originalStart := ui.StartPsDashboard
	defer func() { ui.StartPsDashboard = originalStart }()

	called := false
	ui.StartPsDashboard = func(fetcher ui.SessionFetcher, showCosts bool, sortBy string) error {
		called = true
		assert.True(t, showCosts)
		assert.Equal(t, "cost", sortBy)
		return nil
	}

	// Setup flags
	viper.Set("orchestrator.mode", "local")

	// Reset flags to defaults
	dashboardCmd.Flags().Set("show-costs", "false")
	dashboardCmd.Flags().Set("sort", "time")

	// Set args on rootCmd because dashboard is a subcommand
	rootCmd.SetArgs([]string{"dashboard", "--show-costs", "--sort", "cost"})

	// Execute
	// Note: We need to suppress output/logging during test if needed, but here it's fine.
	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.True(t, called)
}
