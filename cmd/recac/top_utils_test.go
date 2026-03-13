package main

import (
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRunningSessions(t *testing.T) {
	// Setup Mocks
	tmpDir := t.TempDir()

	// Write agent_state.json for one session to test goal extraction
	agentStateContent := `{"history": [{"role": "user", "content": "Write tests.\nSome extra details."}]}`
	agentStateFile := filepath.Join(tmpDir, "agent_state_1.json")
	require.NoError(t, os.WriteFile(agentStateFile, []byte(agentStateContent), 0644))

	mockSM := NewMockSessionManager()
	mockSM.Sessions["session1"] = &runner.SessionState{
		Name:           "session1",
		Status:         "running",
		AgentStateFile: agentStateFile,
	}
	mockSM.Sessions["session2"] = &runner.SessionState{
		Name:   "session2",
		Status: "stopped",
	}
	mockSM.Sessions["session3"] = &runner.SessionState{
		Name:   "session3",
		Status: "running",
		PID:    os.Getpid(), // Use current process to test PID > 0 logic without failing
	}

	origSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
	defer func() { sessionManagerFactory = origSMFactory }()

	cmd := &cobra.Command{}

	sessions, err := getRunningSessions(cmd)
	require.NoError(t, err)

	// Should only return running sessions
	assert.Len(t, sessions, 2)

	// Check session 1
	for _, s := range sessions {
		if s.Name == "session1" {
			assert.Equal(t, "Write tests", s.Goal)
			assert.Equal(t, "N/A", s.CPU)
		} else if s.Name == "session3" {
			// CPU/Mem info should not be N/A
			assert.NotEqual(t, "N/A", s.CPU)
			assert.NotEqual(t, "N/A", s.Memory)
		} else {
			t.Errorf("Unexpected session: %s", s.Name)
		}
	}
}
