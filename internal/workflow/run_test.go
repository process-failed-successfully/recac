package workflow

import (
	"context"
	"testing"

	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
)

// ManualMockSessionManager implements ISessionManager for testing
type ManualMockSessionManager struct {
	StartSessionFunc func(name, goal string, command []string, cwd string) (*runner.SessionState, error)
}

func (m *ManualMockSessionManager) StartSession(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
	if m.StartSessionFunc != nil {
		return m.StartSessionFunc(name, goal, command, cwd)
	}
	return &runner.SessionState{PID: 1234, Name: name, LogFile: "/tmp/mock.log"}, nil
}

func TestRunWorkflow_Detached_Mocked(t *testing.T) {
	// Setup
	mockSM := &ManualMockSessionManager{
		StartSessionFunc: func(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
			assert.Equal(t, "test-detached", name)
			assert.Equal(t, "/tmp/test", cwd)
			// Verify mock flag is passed
			foundMock := false
			for _, arg := range command {
				if arg == "--mock" {
					foundMock = true
					break
				}
			}
			assert.True(t, foundMock, "Expected --mock flag in command")

			return &runner.SessionState{PID: 999, Name: name, LogFile: "test.log"}, nil
		},
	}

	cfg := SessionConfig{
		Detached:       true,
		SessionName:    "test-detached",
		ProjectPath:    "/tmp/test",
		IsMock:         true,
		SessionManager: mockSM,
	}

	// Execute
	err := RunWorkflow(context.Background(), cfg)

	// Assert
	assert.NoError(t, err)
}

func TestRunWorkflow_Detached_MissingName(t *testing.T) {
	cfg := SessionConfig{
		Detached: true,
		// Missing SessionName
	}
	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--name is required")
}

func TestRunWorkflow_Detached_CommandConstruction(t *testing.T) {
	tmpDir := "/tmp/workspace"
	mockSM := &ManualMockSessionManager{
		StartSessionFunc: func(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
			// Check flags
			// Checking specific flags were appended
			foundMaxIter := false
			for i, arg := range command {
				if arg == "--max-iterations" && command[i+1] == "50" {
					foundMaxIter = true
				}
			}
			assert.True(t, foundMaxIter, "Should contain --max-iterations 50")

			foundDirty := false
			for _, arg := range command {
				if arg == "--allow-dirty" {
					foundDirty = true
				}
			}
			assert.True(t, foundDirty, "Should contain --allow-dirty")

			return &runner.SessionState{PID: 1}, nil
		},
	}

	cfg := SessionConfig{
		SessionName:    "test-flags",
		ProjectPath:    tmpDir,
		Detached:       true,
		MaxIterations:  50,
		AllowDirty:     true,
		SessionManager: mockSM,
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.NoError(t, err)
}

func TestRunWorkflow_Detached_ExecutableSearch(t *testing.T) {
	// This tests the executable finding logic.
	mockSM := &ManualMockSessionManager{
		StartSessionFunc: func(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
			// Verify command[0] is not empty
			if len(command) > 0 {
				assert.NotEmpty(t, command[0])
			}
			return &runner.SessionState{PID: 1}, nil
		},
	}

	cfg := SessionConfig{
		SessionName:    "test-exec",
		Detached:       true,
		SessionManager: mockSM,
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.NoError(t, err)
}
