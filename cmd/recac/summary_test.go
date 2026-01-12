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

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestSummaryCommand(t *testing.T) {
	viper.Reset()
	// Setup
	tempDir := t.TempDir()
	// Use t.Setenv to prevent reading the default config file and inheriting its session dir
	t.Setenv("HOME", tempDir)

	sm, err := runner.NewSessionManagerWithDir(filepath.Join(tempDir, ".recac", "sessions"))
	require.NoError(t, err)

	// Override the factory to return our isolated manager
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Create mock sessions
	sessions := []*runner.SessionState{
		{
			Name:      "session-1-recent-cheap",
			Status:    "completed",
			StartTime: time.Now().Add(-1 * time.Hour),
			EndTime:   time.Now().Add(-50 * time.Minute),
			AgentStateFile: createMockAgentState(t, tempDir, "session-1", "model-a", 100, 200),
		},
		{
			Name:      "session-2-expensive",
			Status:    "completed",
			StartTime: time.Now().Add(-2 * time.Hour),
			EndTime:   time.Now().Add(-1 * time.Hour),
			AgentStateFile: createMockAgentState(t, tempDir, "session-2", "model-b", 5000, 10000),
		},
		{
			Name:      "session-3-errored",
			Status:    "error",
			StartTime: time.Now().Add(-3 * time.Hour),
			EndTime:   time.Now().Add(-2 * time.Hour),
		},
		{
			Name:      "session-4-running",
			Status:    "running",
			PID:       os.Getpid(),
			StartTime: time.Now().Add(-10 * time.Minute),
		},
		{
			Name:      "session-5-ancient-expensive",
			Status:    "completed",
			StartTime: time.Now().Add(-100 * time.Hour),
			EndTime:   time.Now().Add(-99 * time.Hour),
			AgentStateFile: createMockAgentState(t, tempDir, "session-5", "model-b", 8000, 12000),
		},
	}

	for _, s := range sessions {
		err := sm.SaveSession(s)
		require.NoError(t, err)
	}

	// Execute command
	cmd, _, _ := newRootCmd()
	cmd.SetArgs([]string{"summary"})
	b := new(bytes.Buffer)
	cmd.SetOut(b)
	err = cmd.Execute()
	require.NoError(t, err)

	output := b.String()

	// Assertions
	t.Run("Aggregate Stats", func(t *testing.T) {
		require.Contains(t, output, "Aggregate Stats")
		require.Regexp(t, `Total Sessions:\s+5`, output)
		require.Regexp(t, `Completed:\s+3`, output)
		require.Regexp(t, `Errored:\s+1`, output)
		require.Regexp(t, `Running:\s+1`, output)
		require.Regexp(t, `Success Rate:\s+60.00%`, output)
	})

	t.Run("Recent Sessions", func(t *testing.T) {
		require.Contains(t, output, "Recent Sessions")
		lines := getSectionLines(output, "Recent Sessions")
		require.Len(t, lines, 5)
		require.Contains(t, lines[0], "session-4-running") // Most recent
		require.Contains(t, lines[1], "session-1-recent-cheap")
		require.Contains(t, lines[2], "session-2-expensive")
		require.Contains(t, lines[3], "session-3-errored")
		require.Contains(t, lines[4], "session-5-ancient-expensive") // Least recent
	})

	t.Run("Most Expensive Sessions", func(t *testing.T) {
		require.Contains(t, output, "Most Expensive Sessions")
		lines := getSectionLines(output, "Most Expensive Sessions")

		require.Len(t, lines, 3) // Only 3 sessions have costs
		require.Contains(t, lines[0], "session-5-ancient-expensive")
		require.Contains(t, lines[1], "session-2-expensive")
		require.Contains(t, lines[2], "session-1-recent-cheap")
	})
}

// createMockAgentState is a helper to create a temporary agent state file.
func createMockAgentState(t *testing.T, dir, sessionName, model string, pTokens, rTokens int) string {
	t.Helper()
	state := &agent.State{
		Model: model,
		TokenUsage: agent.TokenUsage{
			TotalPromptTokens:     pTokens,
			TotalResponseTokens: rTokens,
			TotalTokens:         pTokens + rTokens,
		},
	}
	data, err := json.Marshal(state)
	require.NoError(t, err)

	filePath := filepath.Join(dir, sessionName+"_agent_state.json")
	err = os.WriteFile(filePath, data, 0644)
	require.NoError(t, err)

	return filePath
}

// getSectionLines splits the command output and returns the lines for a specific section.
func getSectionLines(output, sectionTitle string) []string {
	inSection := false
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, sectionTitle) {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "---") {
			continue
		}
		if inSection && line == "" {
			break // End of section
		}
		if inSection {
			lines = append(lines, line)
		}
	}
	// The first line is the header, which we don't need for assertions on content rows.
	if len(lines) > 1 {
		return lines[1:]
	}
	return []string{}
}
