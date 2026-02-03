package ui

import (
	"os"
	"recac/internal/runner"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// AttachMockSessionManager is a mock implementation of runner.ISessionManager
type AttachMockSessionManager struct {
	mock.Mock
}

func (m *AttachMockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	args := m.Called()
	return args.Get(0).([]*runner.SessionState), args.Error(1)
}

func (m *AttachMockSessionManager) SaveSession(session *runner.SessionState) error {
	args := m.Called(session)
	return args.Error(0)
}

func (m *AttachMockSessionManager) LoadSession(name string) (*runner.SessionState, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*runner.SessionState), args.Error(1)
}

func (m *AttachMockSessionManager) StopSession(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *AttachMockSessionManager) PauseSession(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *AttachMockSessionManager) ResumeSession(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *AttachMockSessionManager) GetSessionLogs(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

func (m *AttachMockSessionManager) GetSessionLogContent(name string, lines int) (string, error) {
	args := m.Called(name, lines)
	return args.String(0), args.Error(1)
}

func (m *AttachMockSessionManager) StartSession(name, goal string, command []string, workspace string) (*runner.SessionState, error) {
	args := m.Called(name, goal, command, workspace)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*runner.SessionState), args.Error(1)
}

func (m *AttachMockSessionManager) GetSessionPath(name string) string {
	args := m.Called(name)
	return args.String(0)
}

func (m *AttachMockSessionManager) IsProcessRunning(pid int) bool {
	args := m.Called(pid)
	return args.Bool(0)
}

func (m *AttachMockSessionManager) RemoveSession(name string, force bool) error {
	args := m.Called(name, force)
	return args.Error(0)
}

func (m *AttachMockSessionManager) RenameSession(oldName, newName string) error {
	args := m.Called(oldName, newName)
	return args.Error(0)
}

func (m *AttachMockSessionManager) SessionsDir() string {
	args := m.Called()
	return args.String(0)
}

func (m *AttachMockSessionManager) GetSessionGitDiffStat(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

func (m *AttachMockSessionManager) ArchiveSession(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *AttachMockSessionManager) UnarchiveSession(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *AttachMockSessionManager) ListArchivedSessions() ([]*runner.SessionState, error) {
	args := m.Called()
	return args.Get(0).([]*runner.SessionState), args.Error(1)
}

func TestAttachDashboardModel_Init(t *testing.T) {
	// Create a temp log file
	tmpFile, err := os.CreateTemp("", "test_log_*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write some initial content
	initialContent := `{"time":"2023-10-27T10:00:00Z","level":"INFO","msg":"Starting session"}
{"time":"2023-10-27T10:00:01Z","level":"INFO","msg":"Doing work"}
`
	if _, err := tmpFile.WriteString(initialContent); err != nil {
		t.Fatal(err)
	}

	// Mock session manager
	mockSM := new(AttachMockSessionManager)
	mockSM.On("GetSessionLogs", "test-session").Return(tmpFile.Name(), nil)
	mockSM.On("LoadSession", "test-session").Return(&runner.SessionState{
		Name:   "test-session",
		Status: "running",
		Goal:   "Test Goal",
	}, nil)

	// Create model
	model, err := NewAttachDashboardModel("test-session", mockSM)
	assert.NoError(t, err)
	defer model.file.Close() // Ensure file is closed

	// Test Init
	cmd := model.Init()
	assert.NotNil(t, cmd)

	// Test Update with WindowSizeMsg
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, ok := newModel.(AttachDashboardModel)
	assert.True(t, ok)
	assert.Equal(t, 80, m.width)
	assert.Equal(t, 24, m.height)

	// Test Update with logUpdateMsg using the updated model 'm'
	entries, _ := ParseLogLines([]byte(initialContent))
	newModel, _ = m.Update(logUpdateMsg(entries))
	m, ok = newModel.(AttachDashboardModel)
	assert.True(t, ok)
	assert.Len(t, m.entries, 2)
	assert.Contains(t, m.View(), "Starting session")
	assert.Contains(t, m.View(), "Doing work")

	// Test log reading (simulate update)
	// Write more content
	moreContent := `{"time":"2023-10-27T10:00:02Z","level":"INFO","msg":"Finished work"}
`
	if _, err := tmpFile.WriteString(moreContent); err != nil {
		t.Fatal(err)
	}

	// Create a command to read logs
	readCmd := readLogsCmd(model.file)
	msg := readCmd()
	logMsg, ok := msg.(logUpdateMsg)
	assert.True(t, ok)
	// Since NewAttachDashboardModel seeks to 0 (size < 10000), readLogsCmd reads everything from start to end.
	// We expect 3 entries.
	// Wait, we already called readLogsCmd implicitly? No, Init returns it but we didn't execute it.
	// But `model.file` is shared.
	// Actually, `readLogsCmd` calls `io.ReadAll(file)`. This advances the file offset.
	// If `NewAttachDashboardModel` opened the file, offset is 0.
	// If we call `readLogsCmd` now, it reads everything.
	// BUT, `io.ReadAll` reads until EOF.
	// So subsequent calls will read only new content.

	// Let's verify.
	// First call (simulated)
	// Actually, the previous `model.Update(logUpdateMsg(entries))` did NOT read from file. It just updated the model with dummy entries.
	// The file offset is still at 0 (or wherever NewAttachDashboardModel left it).
	// NewAttachDashboardModel seeks to 0.

	// So first readLogsCmd call should read everything.
	assert.Len(t, logMsg, 3)
}
