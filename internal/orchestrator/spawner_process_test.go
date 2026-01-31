package orchestrator

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"testing"
	"time"

	"recac/internal/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Reusing MockSessionManager, MockPoller, MockGitClient from spawner_docker_test.go
// provided they are available in the same package scope.

func TestProcessSpawner_Spawn(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sm := new(MockSessionManager)
	poller := new(MockPoller)
	gitClient := new(MockGitClient)

	// Mock Expects
	// 1. Initial SaveSession (status: running)
	sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "running" && s.Type == "orchestrated-process"
	})).Return(nil).Once()

	// 2. LoadSession during completion (simulating reloading state)
	sm.On("LoadSession", "TEST-1").Return(&runner.SessionState{Status: "running"}, nil)

	// 3. CurrentCommitSHA call
	gitClient.On("CurrentCommitSHA", mock.Anything).Return("abcdef123456", nil)

	// 4. Final SaveSession (status: completed)
	sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "completed" && s.EndCommitSHA == "abcdef123456"
	})).Return(nil).Once()

	spawner := NewProcessSpawner(logger, "recac-agent", poller, "provider", "model", sm)
	spawner.GitClient = gitClient // Inject mock

	// Override CmdFactory to use echo
	spawner.CmdFactory = func(name string, arg ...string) *exec.Cmd {
		// Just run "echo" which exists on almost all systems
		// It will succeed and exit 0
		return exec.Command("echo", arg...)
	}

	item := WorkItem{
		ID:      "TEST-1",
		RepoURL: "https://github.com/test/repo",
		EnvVars: map[string]string{"FOO": "BAR"},
	}

	err := spawner.Spawn(context.Background(), item)
	assert.NoError(t, err)

	// Wait for the goroutine to likely finish
	time.Sleep(500 * time.Millisecond)

	sm.AssertExpectations(t)
	gitClient.AssertExpectations(t)
}
