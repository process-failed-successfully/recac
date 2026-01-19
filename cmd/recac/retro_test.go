package main

import (
	"context"
	"os"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
)

// RetroMockAgent implements agent.Agent for testing
type RetroMockAgent struct {
	Response string
}

func (m *RetroMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *RetroMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, nil
}

func TestRetroCmd(t *testing.T) {
	// Setup Mock Session Manager
	mockSM := NewMockSessionManager()

	// Create a dummy log file
	tmpLog, err := os.CreateTemp("", "session-*.log")
	assert.NoError(t, err)
	defer os.Remove(tmpLog.Name())

	logContent := "ERROR: Something went wrong\nINFO: Success\n"
	tmpLog.WriteString(logContent)
	tmpLog.Close()

	// Add a session
	mockSM.Sessions["test-session"] = &runner.SessionState{
		Name:      "test-session",
		Status:    "completed",
		StartTime: time.Now(),
		LogFile:   tmpLog.Name(),
	}

	// Override factories
	origSMFactory := sessionManagerFactory
	origAgentFactory := agentClientFactory
	defer func() {
		sessionManagerFactory = origSMFactory
		agentClientFactory = origAgentFactory
	}()

	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &RetroMockAgent{
			Response: "Retrospective Report: It went okay.",
		}, nil
	}

	// Run command
	output, err := executeCommand(rootCmd, "retro", "test-session")
	assert.NoError(t, err)
	assert.Contains(t, output, "Retrospective Report: It went okay.")
}

func TestRetroCmd_NoArgs(t *testing.T) {
	// Setup Mock Session Manager
	mockSM := NewMockSessionManager()

	// Create a dummy log file
	tmpLog, err := os.CreateTemp("", "session-latest-*.log")
	assert.NoError(t, err)
	defer os.Remove(tmpLog.Name())

	tmpLog.WriteString("Latest logs")
	tmpLog.Close()

	// Add sessions (order matters for testing sort)
	mockSM.Sessions["old-session"] = &runner.SessionState{
		Name:      "old-session",
		Status:    "completed",
		StartTime: time.Now().Add(-1 * time.Hour),
		LogFile:   "/dev/null",
	}
	mockSM.Sessions["latest-session"] = &runner.SessionState{
		Name:      "latest-session",
		Status:    "completed",
		StartTime: time.Now(),
		LogFile:   tmpLog.Name(),
	}

	// Override factories
	origSMFactory := sessionManagerFactory
	origAgentFactory := agentClientFactory
	defer func() {
		sessionManagerFactory = origSMFactory
		agentClientFactory = origAgentFactory
	}()

	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &RetroMockAgent{
			Response: "Latest Retro Report",
		}, nil
	}

	// Run command without args
	output, err := executeCommand(rootCmd, "retro")
	assert.NoError(t, err)
	// We expect "Analyzing latest session: latest-session" but executeCommand captures output.
	// Since runRetro writes that to ErrOrStderr, and executeCommand captures it (root.SetErr(b)),
	// we should see it.
	assert.Contains(t, output, "Analyzing latest session: latest-session")
	assert.Contains(t, output, "Latest Retro Report")
}
