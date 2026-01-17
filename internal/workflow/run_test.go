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
