package workflow

import (
	"context"
	"fmt"
	"testing"

	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
)

func TestRunWorkflow_Detached_Success_NewSM(t *testing.T) {
	// Mock NewSessionManagerFunc to cover the branch where cfg.SessionManager is nil
	originalNewSM := NewSessionManagerFunc
	defer func() { NewSessionManagerFunc = originalNewSM }()

	called := false
	NewSessionManagerFunc = func() (ISessionManager, error) {
		return &ManualMockSessionManager{
			StartSessionFunc: func(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
				called = true
				assert.Equal(t, "detached-session", name)
				assert.Equal(t, "test goal", goal)
				assert.Contains(t, command[0], "workflow.test") // The test binary
				return &runner.SessionState{PID: 12345, LogFile: "/tmp/recac.log"}, nil
			},
		}, nil
	}

	cfg := SessionConfig{
		Detached:      true,
		SessionName:   "detached-session",
		Goal:          "test goal",
		ProjectPath:   "/tmp/test-project",
		CommandPrefix: []string{"custom-cmd"},
		// SessionManager is nil here
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.NoError(t, err)
	assert.True(t, called, "StartSession should be called")
}

func TestRunWorkflow_Detached_ManagerError_NewSM(t *testing.T) {
	// Mock NewSessionManagerFunc failure
	originalNewSM := NewSessionManagerFunc
	defer func() { NewSessionManagerFunc = originalNewSM }()

	NewSessionManagerFunc = func() (ISessionManager, error) {
		return nil, fmt.Errorf("manager creation failed")
	}

	cfg := SessionConfig{
		Detached:    true,
		SessionName: "test",
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create session manager")
}
