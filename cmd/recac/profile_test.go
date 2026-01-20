package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgent for Profile Test
type ProfileMockAgent struct {
	mock.Mock
}

func (m *ProfileMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *ProfileMockAgent) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt, callback)
	callback(args.String(0)) // Simulate streaming
	return args.String(0), args.Error(1)
}

func (m *ProfileMockAgent) Close() error {
	return nil
}

// TestHelperProcess is already defined in commands_test.go
// but we need to handle "go tool pprof" which might not be handled there.
// If commands_test.go's version is generic, it might just exit 0.
// Let's check commands_test.go content.

func TestProfileCmd_AnalyzeExisting(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	profFile := filepath.Join(tempDir, "test.prof")
	err := os.WriteFile(profFile, []byte("mock profile data"), 0644)
	assert.NoError(t, err)

	// Mock Agent
	mockAg := new(ProfileMockAgent)
	mockAg.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Return("Analysis: optimize loop", nil)

	// Override factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}

	// Mock execCommand
	origExec := execCommand
	defer func() { execCommand = origExec }()
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	// Prepare Command
	cmd := &cobra.Command{}
	// Use a fresh command to avoid side effects
	// But `runProfile` uses global `profileAnalyzeFile` var.
	// We need to reset it.
	profileAnalyzeFile = profFile
	defer func() { profileAnalyzeFile = "" }()

	// Execute
	err = runProfile(cmd, []string{})
	assert.NoError(t, err)

	// Verify Agent was called
	mockAg.AssertExpectations(t)
}

func TestProfileCmd_RunCommand(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	profFile := filepath.Join(tempDir, "cpu.prof")
	// Pre-create the prof file because our mock helper process won't actually create it,
	// but `runProfile` checks for its existence.
	err := os.WriteFile(profFile, []byte("generated profile"), 0644)
	assert.NoError(t, err)

	// Mock Agent
	mockAg := new(ProfileMockAgent)
	mockAg.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Return("Analysis: optimize loop", nil)

	// Override factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}

	// Mock execCommand
	origExec := execCommand
	defer func() { execCommand = origExec }()
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	// Reset globals
	profileAnalyzeFile = ""
	profileOutput = profFile
	defer func() { profileOutput = "cpu.prof" }()

	// Execute
	cmd := &cobra.Command{}
	// Pass dummy args
	err = runProfile(cmd, []string{"go", "test", "./..."})
	assert.NoError(t, err)

	mockAg.AssertExpectations(t)
}
