package main

import (
	"bytes"
	"os"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// Minimal mock to satisfy ISessionManager interface for GetSessionGitDiffStat
type mockSessionManagerForDisplay struct {
	mockGetSessionGitDiffStat func(name string) (string, error)
}

func (m *mockSessionManagerForDisplay) GetSessionGitDiffStat(name string) (string, error) {
	if m.mockGetSessionGitDiffStat != nil {
		return m.mockGetSessionGitDiffStat(name)
	}
	return "", nil
}

// Stub other methods to panic if called (they shouldn't be for these tests)
func (m *mockSessionManagerForDisplay) ListSessions() ([]*runner.SessionState, error) { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) SaveSession(*runner.SessionState) error { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) LoadSession(name string) (*runner.SessionState, error) { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) StopSession(name string) error { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) PauseSession(name string) error { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) ResumeSession(name string) error { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) GetSessionLogs(name string) (string, error) { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) GetSessionLogContent(name string, lines int) (string, error) { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) StartSession(name, goal string, command []string, workspace string) (*runner.SessionState, error) { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) GetSessionPath(name string) string { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) IsProcessRunning(pid int) bool { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) RemoveSession(name string, force bool) error { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) RenameSession(oldName, newName string) error { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) SessionsDir() string { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) ArchiveSession(name string) error { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) UnarchiveSession(name string) error { panic("unexpected call") }
func (m *mockSessionManagerForDisplay) ListArchivedSessions() ([]*runner.SessionState, error) { panic("unexpected call") }

func TestDisplayStatus(t *testing.T) {
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session := &runner.SessionState{
		Name:      "test-session",
		Status:    "running",
		Goal:      "test goal",
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Time{}, // running
	}

	state := &agent.State{
		Model: "test-model",
		TokenUsage: agent.TokenUsage{
			TotalTokens: 100,
		},
		History: []agent.Message{
			{Role: "user", Content: "hello", Timestamp: time.Now()},
		},
	}

	displayStatus(cmd, session, state)

	output := buf.String()
	assert.Contains(t, output, "test-session")
	assert.Contains(t, output, "running")
	assert.Contains(t, output, "test goal")
	assert.Contains(t, output, "test-model")
	assert.Contains(t, output, "Tokens:")
	assert.Contains(t, output, "100")
	assert.Contains(t, output, "hello")
}

func TestDisplaySessionDetail(t *testing.T) {
	// Mock sessionManagerFactory
	origFactory := sessionManagerFactory
	defer func() { sessionManagerFactory = origFactory }()

	sessionManagerFactory = func() (ISessionManager, error) {
		return &mockSessionManagerForDisplay{
			mockGetSessionGitDiffStat: func(name string) (string, error) {
				return " 1 file changed", nil
			},
		}, nil
	}

	// Mock loadAgentState if needed, but here we don't set AgentStateFile in session, so it skips it.
	// We can test with AgentStateFile too.
	origLoadState := loadAgentState
	defer func() { loadAgentState = origLoadState }()
	loadAgentState = func(path string) (*agent.State, error) {
		return &agent.State{
			Model: "mock-model",
			TokenUsage: agent.TokenUsage{TotalTokens: 50},
		}, nil
	}

	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session := &runner.SessionState{
		Name:           "test-session",
		Status:         "completed",
		StartTime:      time.Now().Add(-1 * time.Hour),
		LogFile:        "test.log",
		AgentStateFile: "agent.json",
	}

	// Create dummy log file
	os.WriteFile("test.log", []byte("log line 1\nlog line 2"), 0644)
	defer os.Remove("test.log")

	err := DisplaySessionDetail(cmd, session, false)
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Session Details")
	assert.Contains(t, output, "test-session")
	assert.Contains(t, output, "Git Changes")
	assert.Contains(t, output, "1 file changed")
	assert.Contains(t, output, "log line 1")
	assert.Contains(t, output, "mock-model")
}

func TestDisplaySessionDiff(t *testing.T) {
	// Mock loadAgentState
	origLoadState := loadAgentState
	defer func() { loadAgentState = origLoadState }()
	loadAgentState = func(path string) (*agent.State, error) {
		return &agent.State{
			Model: "mock-model",
			TokenUsage: agent.TokenUsage{TotalTokens: 50},
		}, nil
	}

	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	sessionA := &runner.SessionState{
		Name:           "session-A",
		Status:         "completed",
		LogFile:        "logA.log",
		AgentStateFile: "agentA.json",
	}
	sessionB := &runner.SessionState{
		Name:           "session-B",
		Status:         "completed",
		LogFile:        "logB.log",
		AgentStateFile: "agentB.json",
	}

	// Create dummy logs
	os.WriteFile("logA.log", []byte("line 1\nline 2"), 0644)
	defer os.Remove("logA.log")
	os.WriteFile("logB.log", []byte("line 1\nline 3"), 0644)
	defer os.Remove("logB.log")

	err := DisplaySessionDiff(cmd, sessionA, sessionB)
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Metadata Comparison")
	assert.Contains(t, output, "session-A")
	assert.Contains(t, output, "session-B")
	assert.Contains(t, output, "mock-model") // from mocked agent state

	// Check log diff output
	// Since we might use fallbackDiff or real diff, check for common indicators
	// "line 2" should be removed (-), "line 3" added (+)
	// Or at least they appear in output.
	assert.Contains(t, output, "line 2")
	assert.Contains(t, output, "line 3")
}

func TestFallbackDiff(t *testing.T) {
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Create dummy files
	os.WriteFile("f1.txt", []byte("line 1\nline 2"), 0644)
	defer os.Remove("f1.txt")
	os.WriteFile("f2.txt", []byte("line 1\nline 3"), 0644)
	defer os.Remove("f2.txt")

	err := fallbackDiff(cmd, "f1.txt", "f2.txt")
	assert.NoError(t, err)

	output := buf.String()
	// Fallback diff output format check
	assert.Contains(t, output, "- line 2")
	assert.Contains(t, output, "+ line 3")
}
