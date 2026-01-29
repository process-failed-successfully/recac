package workflow

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ExtendedMockSessionManager is a mock implementation of ISessionManager
type ExtendedMockSessionManager struct {
	mock.Mock
}

func (m *ExtendedMockSessionManager) StartSession(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
	args := m.Called(name, goal, command, cwd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*runner.SessionState), args.Error(1)
}

func TestRunWorkflow_Detached_Mock(t *testing.T) {
	mockSM := new(ExtendedMockSessionManager)

	tmpDir := t.TempDir()
	// No need to create "recac" executable if we mock SessionManager and Detached logic doesn't strictly check for "recac" presence
	// before calling SessionManager IF we are running tests.
	// But `RunWorkflow` does: `executable, err := os.Executable()`.
	// And then it might append `start`.

	cfg := SessionConfig{
		Detached:       true,
		SessionName:    "test-detached",
		Goal:           "goal",
		ProjectPath:    tmpDir,
		SessionManager: mockSM,
	}

	expectedState := &runner.SessionState{
		PID:     123,
		LogFile: "test.log",
	}

	mockSM.On("StartSession", "test-detached", "goal", mock.Anything, tmpDir).Return(expectedState, nil)

	err := RunWorkflow(context.Background(), cfg)
	assert.NoError(t, err)
	mockSM.AssertExpectations(t)
}

func TestRunWorkflow_Detached_Error_Extended(t *testing.T) {
	mockSM := new(ExtendedMockSessionManager)
	tmpDir := t.TempDir()

	cfg := SessionConfig{
		Detached:       true,
		SessionName:    "test-detached-err",
		Goal:           "goal",
		ProjectPath:    tmpDir,
		SessionManager: mockSM,
	}

	mockSM.On("StartSession", "test-detached-err", "goal", mock.Anything, tmpDir).Return(nil, fmt.Errorf("start failed"))

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start failed")
	mockSM.AssertExpectations(t)
}

func TestRunWorkflow_Detached_NoName(t *testing.T) {
	cfg := SessionConfig{
		Detached: true,
		// No SessionName
	}
	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--name is required")
}

func TestRunWorkflow_MockMode(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg := SessionConfig{
		IsMock:        true,
		ProjectPath:   tmpDir,
		SessionName:   "test-mock",
		MaxIterations: 1,
		Debug:         true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = RunWorkflow(ctx, cfg)

	if err != nil {
		assert.Contains(t, err.Error(), "maximum iterations reached")
	}
}

func TestRunWorkflow_PreFlight_Dirty(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	tmpDir := t.TempDir()

	runCmd := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("command failed: %s %v: %v", name, args, err)
		}
	}

	runCmd("git", "init")
	runCmd("git", "config", "user.email", "test@example.com")
	runCmd("git", "config", "user.name", "Test User")

	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644)
	runCmd("git", "add", ".")
	runCmd("git", "commit", "-m", "init")

	// Make dirty
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("dirty"), 0644)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "test-dirty",
		AllowDirty:  false,
		IsMock:      false,
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes detected")
}

func TestRunWorkflow_Detached_NewSessionManagerFail(t *testing.T) {
	// Mock NewSessionManagerFunc failure
	originalFunc := NewSessionManagerFunc
	defer func() { NewSessionManagerFunc = originalFunc }()
	NewSessionManagerFunc = func() (ISessionManager, error) {
		return nil, fmt.Errorf("mock init failed")
	}

	cfg := SessionConfig{
		Detached:    true,
		SessionName: "test-detached-fail",
		Goal:        "goal",
		ProjectPath: t.TempDir(),
		// SessionManager is nil, so it calls factory
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mock init failed")
}

func TestRunWorkflow_AgentInitFail(t *testing.T) {
	// Mock cmdutils.GetAgentClient failure
	originalGetAgentClient := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, fmt.Errorf("mock agent init failed")
	}

	cfg := SessionConfig{
		SessionName: "test-agent-fail",
		ProjectPath: t.TempDir(),
		IsMock:      false,
		AllowDirty:  true, // skip git check
	}

	// Create app_spec.txt to pass pre-checks if any (though AllowDirty skips git)
	os.WriteFile(filepath.Join(cfg.ProjectPath, "app_spec.txt"), []byte("spec"), 0644)

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mock agent init failed")
}
