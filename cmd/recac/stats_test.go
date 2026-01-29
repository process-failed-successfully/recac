package main

import (
	"bytes"
	"errors"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"testing"
)

// MockSessionManager for stats test
type mockSessionManager struct {
	sessions []*runner.SessionState
	err      error
}

func (m *mockSessionManager) StartSession(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
	return nil, nil
}
func (m *mockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	return m.sessions, m.err
}
func (m *mockSessionManager) StopSession(name string) error { return nil }
func (m *mockSessionManager) SaveSession(*runner.SessionState) error { return nil }
func (m *mockSessionManager) LoadSession(name string) (*runner.SessionState, error) { return nil, nil }
func (m *mockSessionManager) PauseSession(name string) error { return nil }
func (m *mockSessionManager) ResumeSession(name string) error { return nil }
func (m *mockSessionManager) GetSessionLogs(name string) (string, error) { return "", nil }
func (m *mockSessionManager) GetSessionLogContent(name string, lines int) (string, error) { return "", nil }
func (m *mockSessionManager) GetSessionPath(name string) string { return "" }
func (m *mockSessionManager) IsProcessRunning(pid int) bool { return false }
func (m *mockSessionManager) RemoveSession(name string, force bool) error { return nil }
func (m *mockSessionManager) RenameSession(oldName, newName string) error { return nil }
func (m *mockSessionManager) SessionsDir() string { return "" }
func (m *mockSessionManager) GetSessionGitDiffStat(name string) (string, error) { return "", nil }
func (m *mockSessionManager) ArchiveSession(name string) error { return nil }
func (m *mockSessionManager) UnarchiveSession(name string) error { return nil }
func (m *mockSessionManager) ListArchivedSessions() ([]*runner.SessionState, error) { return nil, nil }


func TestCalculateStats(t *testing.T) {
	// Mock loadAgentState
	originalLoad := loadAgentState
	defer func() { loadAgentState = originalLoad }()

	loadAgentState = func(path string) (*agent.State, error) {
		if path == "error" {
			return nil, errors.New("fail")
		}
		if path == "empty" {
			return &agent.State{}, nil // empty
		}
		// return dummy state
		return &agent.State{
			Model: "test-model",
			TokenUsage: agent.TokenUsage{
				TotalTokens: 100,
				TotalPromptTokens: 80,
				TotalResponseTokens: 20,
			},
		}, nil
	}

	tests := []struct {
		name    string
		sessions []*runner.SessionState
		listErr error
		wantErr bool
		validate func(*testing.T, *AggregateStats)
	}{
		{
			name: "Success",
			sessions: []*runner.SessionState{
				{Name: "s1", Status: "running", AgentStateFile: "valid"},
				{Name: "s2", Status: "completed", AgentStateFile: "valid"},
				{Name: "s3", Status: "running", AgentStateFile: ""}, // No state file
			},
			validate: func(t *testing.T, s *AggregateStats) {
				if s.TotalSessions != 3 {
					t.Errorf("Expected 3 sessions, got %d", s.TotalSessions)
				}
				if s.TotalTokens != 200 { // 100 * 2
					t.Errorf("Expected 200 tokens, got %d", s.TotalTokens)
				}
				if s.StatusCounts["running"] != 2 {
					t.Errorf("Expected 2 running, got %d", s.StatusCounts["running"])
				}
			},
		},
		{
			name: "List Error",
			listErr: errors.New("list fail"),
			wantErr: true,
		},
		{
			name: "Load State Error",
			sessions: []*runner.SessionState{
				{Name: "s1", AgentStateFile: "error"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := &mockSessionManager{
				sessions: tt.sessions,
				err:      tt.listErr,
			}
			stats, err := calculateStats(sm)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.validate != nil {
					tt.validate(t, stats)
				}
			}
		})
	}
}

func TestDisplayStats(t *testing.T) {
	stats := &AggregateStats{
		TotalSessions:       10,
		TotalTokens:         1000,
		TotalPromptTokens:   800,
		TotalResponseTokens: 200,
		TotalCost:           0.1234,
		StatusCounts: map[string]int{
			"running": 5,
			"done":    5,
		},
	}

	var buf bytes.Buffer
	displayStats(&buf, stats)

	out := buf.String()
	if !strings.Contains(out, "Total Sessions:   10") {
		t.Errorf("Output mismatch: %s", out)
	}
	if !strings.Contains(out, "$0.1234") {
		t.Errorf("Cost mismatch: %s", out)
	}
	if !strings.Contains(out, "running") || !strings.Contains(out, "5") {
		t.Error("Missing status breakdown")
	}
}
