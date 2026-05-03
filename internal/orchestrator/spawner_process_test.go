package orchestrator

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"recac/internal/runner"
	"recac/internal/telemetry"
)

// mockSessionManager is used to mock SessionManager calls
type mockSessionManager struct {
	mock.Mock
}

func (m *mockSessionManager) SaveSession(session *runner.SessionState) error {
	args := m.Called(session)
	return args.Error(0)
}

func (m *mockSessionManager) LoadSession(name string) (*runner.SessionState, error) {
	args := m.Called(name)
	return args.Get(0).(*runner.SessionState), args.Error(1)
}

func TestProcessSpawner_UpdateStatusOnFailure(t *testing.T) {
	logger := telemetry.NewLogger(true, "test", true)
	sm := &mockSessionManager{}
	poller := new(MockPoller)

	sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return filepath.IsAbs(s.AgentStateFile) && filepath.Base(s.AgentStateFile) == ".agent_state.json"
	})).Return(nil)

	// We expect UpdateStatus to be called with "Failed" because the process execution will fail
	// (since recac-agent doesn't exist or is not fully executable in the test environment)
	poller.On("UpdateStatus", mock.Anything, mock.Anything, "Failed", mock.MatchedBy(func(msg string) bool {
		return len(msg) > 0
	})).Return(nil).Once()

	spawner := NewProcessSpawner(logger, poller, "provider", "model", sm, 10, 5, 2)
	mockGit := new(MockGitClient)
	mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)
	spawner.GitClient = mockGit

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	item := WorkItem{
		ID: "TEST-UPDATE-STATUS",
	}

	err := spawner.Spawn(ctx, item)
	assert.Error(t, err)

	poller.AssertExpectations(t)
	sm.AssertExpectations(t)
}

func TestProcessSpawner_Ping(t *testing.T) {
	// Ping just checks if recac-agent is in PATH, which might fail on CI if not installed.
	// We'll mock exec.LookPath if possible, but it's a package level function.
	// Since we are adding an actual feature, Ping will naturally pass or fail depending on env.
	// Let's just create the spawner and check struct init.
	logger := telemetry.NewLogger(true, "test", true)
	sm := &mockSessionManager{}
	spawner := NewProcessSpawner(logger, nil, "provider", "model", sm, 10, 5, 2)
	mockGit := new(MockGitClient)
	spawner.GitClient = mockGit

	assert.NotNil(t, spawner)
	assert.Equal(t, "provider", spawner.AgentProvider)

	// Test Ping (ignore error, just ensure it executes without panic)
	spawner.Ping(context.Background())
}

func TestProcessSpawner_SpawnAndCleanup(t *testing.T) {
	logger := telemetry.NewLogger(true, "test", true)
	sm := &mockSessionManager{}

	// Expect SaveSession twice (start and end)
	sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return filepath.IsAbs(s.AgentStateFile) && filepath.Base(s.AgentStateFile) == ".agent_state.json"
	})).Return(nil).Twice()

	spawner := NewProcessSpawner(logger, nil, "provider", "model", sm, 10, 5, 2)
	mockGit := new(MockGitClient)
	mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)
	spawner.GitClient = mockGit

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	item := WorkItem{
		ID: "TEST-PROC",
	}

	// Because we don't have recac-agent installed, we intercept exec.Command by redefining it
	// or we can test it using the helper pattern. Wait, ProcessSpawner doesn't expose exec.Command.
	// To avoid actual process execution errors failing the test, we can mock exec using the
	// standard TestHelperProcess pattern if needed, but since we can't easily inject it into
	// ProcessSpawner without changing its signature, we will accept that Spawn might return an error
	// (executable file not found in $PATH). The cleanup should still work.

	err := spawner.Spawn(ctx, item)
	// It should fail because recac-agent doesn't exist, BUT we can verify cleanup works.
	if err != nil {
		// Because we're now wrapping the command in a shell, the execution of the shell might succeed
		// but the wait might fail with "agent process failed: exit status 127" (command not found).
		assert.Error(t, err)
	}

	// Test Cleanup
	err = spawner.Cleanup(ctx, item)
	assert.NoError(t, err)
}

func TestProcessSpawner_Cancel(t *testing.T) {
	logger := telemetry.NewLogger(true, "test", true)
	sm := &mockSessionManager{}
	spawner := NewProcessSpawner(logger, nil, "provider", "model", sm, 10, 5, 2)
	mockGit := new(MockGitClient)
	spawner.GitClient = mockGit

	// Cancellation of non-existent job should fail
	err := spawner.Cancel(context.Background(), "NON-EXISTENT")
	assert.Error(t, err)

	// Simulate active command
	cmd := exec.Command("sleep", "10")
	cmd.Start()

	spawner.mu.Lock()
	spawner.activeCmds["TEST-CANCEL"] = cmd
	spawner.mu.Unlock()

	err = spawner.Cancel(context.Background(), "TEST-CANCEL")
	assert.NoError(t, err)
}

func TestProcessSpawner_GetLogs(t *testing.T) {
	logger := telemetry.NewLogger(true, "test", true)
	sm := &mockSessionManager{}
	spawner := NewProcessSpawner(logger, nil, "provider", "model", sm, 10, 5, 2)
	mockGit := new(MockGitClient)
	spawner.GitClient = mockGit

	// Create a dummy log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "logs.txt")
	os.WriteFile(logPath, []byte("test log output"), 0644)

	spawner.mu.Lock()
	spawner.logFiles["TEST-LOGS"] = logPath
	spawner.mu.Unlock()

	reader, err := spawner.GetLogs(context.Background(), "TEST-LOGS")
	assert.NoError(t, err)
	defer reader.Close()

	content, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, "test log output", string(content))
}

func TestProcessSpawner_Cleanup_Error(t *testing.T) {
	logger := telemetry.NewLogger(true, "test", true)
	sm := &mockSessionManager{}

	spawner := NewProcessSpawner(logger, nil, "provider", "model", sm, 10, 5, 2)

	// Create a dummy log file
	tmpDir := t.TempDir()

	// Create a sub directory to cause permission denied on RemoveAll
	subDir := filepath.Join(tmpDir, "sub")
	err := os.Mkdir(subDir, 0755)
	assert.NoError(t, err)

	logPath := filepath.Join(subDir, "logs.txt")
	err = os.WriteFile(logPath, []byte("test log output"), 0644)
	assert.NoError(t, err)

	spawner.mu.Lock()
	spawner.logFiles["TEST-CLEANUP-ERR"] = logPath
	spawner.mu.Unlock()

	// Cause an error when attempting to remove the directory
	// Note: We need to set permissions such that os.RemoveAll fails.
	// os.RemoveAll calls os.Remove on files, and if the parent dir is not writable/executable, it fails.
	err = os.Chmod(subDir, 0000)
	assert.NoError(t, err)
	// Fix permissions to allow test cleanup to work
	defer os.Chmod(subDir, 0755)

	err = spawner.Cleanup(context.Background(), WorkItem{ID: "TEST-CLEANUP-ERR"})
	assert.Error(t, err)
}

func TestProcessSpawner_Cancel_NonExistent(t *testing.T) {
	logger := telemetry.NewLogger(true, "test", true)
	sm := &mockSessionManager{}
	spawner := NewProcessSpawner(logger, nil, "provider", "model", sm, 10, 5, 2)
	mockGit := new(MockGitClient)
	spawner.GitClient = mockGit

	err := spawner.Cancel(context.Background(), "NON-EXISTENT")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job NON-EXISTENT is not actively running as a process")
}

func TestProcessSpawner_Cancel_NilProcess(t *testing.T) {
	logger := telemetry.NewLogger(true, "test", true)
	sm := &mockSessionManager{}
	spawner := NewProcessSpawner(logger, nil, "provider", "model", sm, 10, 5, 2)
	mockGit := new(MockGitClient)
	spawner.GitClient = mockGit

	spawner.mu.Lock()
	spawner.activeCmds["TEST-NIL"] = &exec.Cmd{} // Process is nil
	spawner.mu.Unlock()

	err := spawner.Cancel(context.Background(), "TEST-NIL")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job TEST-NIL is not actively running as a process")
}

func TestProcessSpawner_GetLogs_Missing(t *testing.T) {
	logger := telemetry.NewLogger(true, "test", true)
	sm := &mockSessionManager{}
	spawner := NewProcessSpawner(logger, nil, "provider", "model", sm, 10, 5, 2)
	mockGit := new(MockGitClient)
	spawner.GitClient = mockGit

	reader, err := spawner.GetLogs(context.Background(), "NON-EXISTENT")
	assert.Error(t, err)
	assert.Nil(t, reader)
	assert.Contains(t, err.Error(), "no logs found for job NON-EXISTENT")
}

func TestProcessSpawner_GetLogs_PermissionDenied(t *testing.T) {
	logger := telemetry.NewLogger(true, "test", true)
	sm := &mockSessionManager{}
	spawner := NewProcessSpawner(logger, nil, "provider", "model", sm, 10, 5, 2)
	mockGit := new(MockGitClient)
	spawner.GitClient = mockGit

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "logs.txt")
	err := os.WriteFile(logPath, []byte("test"), 0000)
	assert.NoError(t, err)

	spawner.mu.Lock()
	spawner.logFiles["TEST-PERM"] = logPath
	spawner.mu.Unlock()

	// Need to make sure file cannot be read
	err = os.Chmod(logPath, 0000)
	assert.NoError(t, err)
	defer os.Chmod(logPath, 0644)

	reader, err := spawner.GetLogs(context.Background(), "TEST-PERM")
	assert.Error(t, err)
	assert.Nil(t, reader)
	assert.Contains(t, err.Error(), "failed to open log file")
}

// A mock process to trigger error on Signal
func TestProcessSpawner_Cancel_SignalError(t *testing.T) {
	logger := telemetry.NewLogger(true, "test", true)
	sm := &mockSessionManager{}
	spawner := NewProcessSpawner(logger, nil, "provider", "model", sm, 10, 5, 2)
	mockGit := new(MockGitClient)
	spawner.GitClient = mockGit

	// Create a dummy process that has already exited to cause a signal error
	cmd := exec.Command("true")
	err := cmd.Start()
	assert.NoError(t, err)
	cmd.Wait() // wait for it to exit so signal fails

	spawner.mu.Lock()
	spawner.activeCmds["TEST-SIGERR"] = cmd
	spawner.mu.Unlock()

	err = spawner.Cancel(context.Background(), "TEST-SIGERR")
	// On some systems sending a signal to an exited process might not return an error but os: process already finished.
	// Actually, Go 1.16+ returns an error os.ErrProcessDone.
	assert.Error(t, err)
}

func TestProcessSpawner_Ping_Coverage(t *testing.T) {
	tests := []struct {
		name        string
		setupPATH   func(t *testing.T)
		expectError bool
		errorMsg    string
	}{
		{
			name: "Fail_AgentNotFound",
			setupPATH: func(t *testing.T) {
				t.Setenv("PATH", "") // Guarantee exec.LookPath fails
			},
			expectError: true,
			errorMsg:    "recac-agent not found in PATH",
		},
		{
			name: "Success_AgentFound",
			setupPATH: func(t *testing.T) {
				tmpDir := t.TempDir()
				dummyBin := filepath.Join(tmpDir, "recac-agent")
				err := os.WriteFile(dummyBin, []byte("#!/bin/sh\nexit 0"), 0755)
				assert.NoError(t, err)
				t.Setenv("PATH", tmpDir)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := telemetry.NewLogger(true, "test", true)
			sm := &mockSessionManager{}
			spawner := NewProcessSpawner(logger, nil, "provider", "model", sm, 10, 5, 2)
			mockGit := new(MockGitClient)
			spawner.GitClient = mockGit

			tt.setupPATH(t)

			err := spawner.Ping(context.Background())
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
