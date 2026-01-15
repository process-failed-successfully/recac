package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"
	"recac/pkg/mocks"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummaryCommand(t *testing.T) {
	t.Run("displays summary for the last 24 hours", func(t *testing.T) {
		// --- Setup ---
		tempDir := t.TempDir()
		mockSM := setupSummaryTest(t, tempDir, []runner.SessionState{
			{Name: "session-1", Status: "completed", StartTime: time.Now().Add(-1 * time.Hour)},
			{Name: "session-2", Status: "error", StartTime: time.Now().Add(-2 * time.Hour)},
			{Name: "session-3", Status: "running", StartTime: time.Now().Add(-3 * time.Hour)},
			{Name: "session-old", Status: "completed", StartTime: time.Now().Add(-48 * time.Hour)},
		})
		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSM, nil
		}
		defer func() { sessionManagerFactory = nil }()

		// --- Execute ---
		cmd := newSummaryCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)
		cmd.SetArgs([]string{"--since", "24h"})
		err := cmd.Execute()

		// --- Assert ---
		require.NoError(t, err)
		output := b.String()
		assert.Contains(t, output, "SESSION SUMMARY (Last 24h)")
		assert.Contains(t, output, "Total Sessions:   3")
		assert.Contains(t, output, "  - Successful:   1")
		assert.Contains(t, output, "  - Failed:       1")
		assert.Contains(t, output, "  - Running:      1")
	})

	t.Run("handles no recent sessions", func(t *testing.T) {
		// --- Setup ---
		tempDir := t.TempDir()
		mockSM := setupSummaryTest(t, tempDir, []runner.SessionState{
			{Name: "session-old-1", Status: "completed", StartTime: time.Now().Add(-72 * time.Hour)},
			{Name: "session-old-2", Status: "error", StartTime: time.Now().Add(-96 * time.Hour)},
		})
		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSM, nil
		}
		defer func() { sessionManagerFactory = nil }()

		// --- Execute ---
		cmd := newSummaryCmd()
		b := new(bytes.Buffer)
		cmd.SetOut(b)
		cmd.SetArgs([]string{"--since", "24h"})
		err := cmd.Execute()

		// --- Assert ---
		require.NoError(t, err)
		output := b.String()
		assert.Contains(t, output, "Total Sessions:   0")
	})

	t.Run("calculates success rate correctly", func(t *testing.T) {
		// --- Setup ---
		tempDir := t.TempDir()
		mockSM := setupSummaryTest(t, tempDir, []runner.SessionState{
			{Name: "s1", Status: "completed", StartTime: time.Now()},
			{Name: "s2", Status: "completed", StartTime: time.Now()},
			{Name: "s3", Status: "completed", StartTime: time.Now()},
			{Name: "s4", Status: "error", StartTime: time.Now()},
			{Name: "s5", Status: "running", StartTime: time.Now()}, // Running sessions don't count towards success rate
		})
		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSM, nil
		}
		defer func() { sessionManagerFactory = nil }()

		// --- Execute ---
		summary, err := calculateSummary(mockSM, "1h")
		require.NoError(t, err)

		// --- Assert ---
		assert.Equal(t, 5, summary.TotalSessions)
		assert.Equal(t, 3, summary.SuccessfulSessions)
		assert.Equal(t, 1, summary.FailedSessions)
		assert.InDelta(t, 75.00, summary.SuccessRate, 0.01)
	})
}

// newSummaryCmd creates a fresh instance of the summary command for testing.
func newSummaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "summary",
		RunE: summaryCmd.RunE,
	}
	cmd.Flags().String("since", "24h", "")
	return cmd
}

// setupSummaryTest initializes a mock session manager with predefined session states.
func setupSummaryTest(t *testing.T, tempDir string, sessions []runner.SessionState) *mocks.MockSessionManager {
	mockSM := &mocks.MockSessionManager{
		Sessions: make(map[string]*runner.SessionState),
	}

	for i, s := range sessions {
		sessionCopy := s
		// Create a dummy agent state file for each session to allow cost calculation
		agentState := agent.State{
			Model: "test-model",
			TokenUsage: agent.TokenUsage{
				TotalPromptTokens:   100,
				TotalResponseTokens: 200,
				TotalTokens:         300,
			},
		}
		stateFilePath := filepath.Join(tempDir, fmt.Sprintf("agent-state-%d.json", i))
		stateManager := agent.NewStateManager(stateFilePath)
		err := stateManager.Save(agentState)
		require.NoError(t, err)
		sessionCopy.AgentStateFile = stateFilePath

		mockSM.Sessions[s.Name] = &sessionCopy
	}

	return mockSM
}
