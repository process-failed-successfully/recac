package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockSessionManager is a mock that implements ISessionManager for testing.
type MockSessionManager struct {
	Sessions []*runner.SessionState
	Err      error
}

// ListSessions returns the mock sessions or an error.
func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	return m.Sessions, m.Err
}

// The following methods are not used in the history command tests, so they
// have minimal implementations to satisfy the interface.
func (m *MockSessionManager) SaveSession(s *runner.SessionState) error    { return nil }
func (m *MockSessionManager) LoadSession(name string) (*runner.SessionState, error) { return nil, nil }
func (m *MockSessionManager) StopSession(name string) error                 { return nil }
func (m *MockSessionManager) GetSessionLogs(name string) (string, error)    { return "", nil }
func (m *MockSessionManager) StartSession(name string, command []string, workspace string) (*runner.SessionState, error) {
	return nil, nil
}
func (m *MockSessionManager) GetSessionPath(name string) string { return "" }

// setupTestSessions creates a slice of sessions with varying properties for testing.
func setupTestSessions(t *testing.T) []*runner.SessionState {
	t.Helper()
	// createAgentState creates a temporary agent state file and returns its path.
	createAgentState := func(name string, promptTokens, responseTokens int) string {
		tmpDir := t.TempDir()
		state := agent.State{
			Model: "gemini-pro", // Use a model with known pricing
			TokenUsage: agent.TokenUsage{
				TotalPromptTokens:   promptTokens,
				TotalResponseTokens: responseTokens,
				TotalTokens:         promptTokens + responseTokens,
			},
		}
		filePath := filepath.Join(tmpDir, name+"_agent_state.json")
		data, err := json.Marshal(state)
		require.NoError(t, err)
		err = os.WriteFile(filePath, data, 0644)
		require.NoError(t, err)
		return filePath
	}

	// Cost = (100k/1M * 0.5) + (300k/1M * 1.5) = 0.05 + 0.45 = $0.50. Tokens = 400k
	sessHigh := &runner.SessionState{
		Name:           "sess-completed-high",
		Status:         "completed",
		StartTime:      time.Now().Add(-1 * time.Hour),
		AgentStateFile: createAgentState("sess-completed-high", 100000, 300000),
	}
	// Cost = (50k/1M * 0.5) + (50k/1M * 1.5) = 0.025 + 0.075 = $0.10. Tokens = 100k
	sessLow := &runner.SessionState{
		Name:           "sess-completed-low",
		Status:         "completed",
		StartTime:      time.Now().Add(-2 * time.Hour),
		AgentStateFile: createAgentState("sess-completed-low", 50000, 50000),
	}
	// Cost = (400k/1M * 0.5) + (0/1M * 1.5) = $0.20. Tokens = 400k
	sessFailed := &runner.SessionState{
		Name:           "sess-failed",
		Status:         "failed",
		StartTime:      time.Now().Add(-3 * time.Hour),
		AgentStateFile: createAgentState("sess-failed", 400000, 0),
	}
	// Should be ignored by the history command
	sessRunning := &runner.SessionState{
		Name:           "sess-running",
		Status:         "running",
		StartTime:      time.Now(),
		AgentStateFile: createAgentState("sess-running", 1000, 1000),
	}

	return []*runner.SessionState{sessHigh, sessLow, sessFailed, sessRunning}
}

// checkStringOrder asserts that a set of substrings appear in a specific order within a larger string.
func checkStringOrder(t *testing.T, content string, substrings []string) {
	t.Helper()
	lastIndex := -1
	for _, sub := range substrings {
		index := strings.Index(content, sub)
		require.Greater(t, index, -1, "substring '%s' not found in output", sub)
		require.Greater(t, index, lastIndex, "substring '%s' appeared out of order", sub)
		lastIndex = index
	}
}

func TestHistoryCmd(t *testing.T) {
	allSessions := setupTestSessions(t)

	testCases := []struct {
		name             string
		statusFilter     string
		sortBy           string
		limit            int
		mockSessions     []*runner.SessionState
		expectedNames    []string // Expected session names in the output. Order matters if checkOrder is true.
		unexpectedNames  []string // Session names that should NOT be in the output.
		checkOrder       bool     // Whether to check the order of expectedNames.
		expectedContains string   // A simple string to check for, like an error message.
	}{
		{
			name:            "No Filters",
			mockSessions:    allSessions,
			expectedNames:   []string{"sess-completed-high", "sess-completed-low", "sess-failed"},
			unexpectedNames: []string{"sess-running"},
			checkOrder:      false,
		},
		{
			name:            "Filter by Status 'completed'",
			statusFilter:    "completed",
			mockSessions:    allSessions,
			expectedNames:   []string{"sess-completed-high", "sess-completed-low"},
			unexpectedNames: []string{"sess-failed", "sess-running"},
			checkOrder:      false,
		},
		{
			name:         "Sort by Cost (Descending)",
			sortBy:       "cost",
			mockSessions: allSessions,
			// Order: high ($0.50), failed ($0.20), low ($0.10)
			expectedNames:   []string{"sess-completed-high", "sess-failed", "sess-completed-low"},
			unexpectedNames: []string{"sess-running"},
			checkOrder:      true,
		},
		{
			name:         "Sort by Tokens (Descending)",
			sortBy:       "tokens",
			mockSessions: allSessions,
			// Order: high (400k), failed (400k), low (100k)
			// Order between high and failed is not guaranteed as they have the same token count
			expectedNames:   []string{"400000", "100000"},
			unexpectedNames: []string{"sess-running"},
			checkOrder:      true,
		},
		{
			name:         "Limit to 1",
			limit:        1,
			mockSessions: allSessions,
			// Default sort is by start time, so the most recent non-running session is 'sess-completed-high'
			expectedNames:   []string{"sess-completed-high"},
			unexpectedNames: []string{"sess-completed-low", "sess-failed", "sess-running"},
		},
		{
			name:         "Sort by Cost and Limit to 1",
			sortBy:       "cost",
			limit:        1,
			mockSessions: allSessions,
			expectedNames:   []string{"sess-completed-high"}, // The most expensive one
			unexpectedNames: []string{"sess-completed-low", "sess-failed", "sess-running"},
		},
		{
			name:             "No Sessions Found",
			mockSessions:     []*runner.SessionState{},
			expectedContains: "No completed sessions found.",
		},
		{
			name:             "No Matching Sessions",
			statusFilter:     "nonexistent",
			mockSessions:     allSessions,
			expectedContains: "No completed sessions found.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup command and buffer
			cmd := &cobra.Command{}
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)

			// Setup mock
			mockSM := &MockSessionManager{Sessions: tc.mockSessions}

			// Execute the command logic
			err := runHistoryCmd(cmd, mockSM, tc.statusFilter, tc.sortBy, tc.limit)
			require.NoError(t, err)
			output := buf.String()

			// Assertions
			if tc.expectedContains != "" {
				assert.Contains(t, output, tc.expectedContains)
			}

			for _, name := range tc.expectedNames {
				assert.Contains(t, output, name)
			}

			for _, name := range tc.unexpectedNames {
				assert.NotContains(t, output, name)
			}

			if tc.checkOrder {
				checkStringOrder(t, output, tc.expectedNames)
			}
		})
	}
}
