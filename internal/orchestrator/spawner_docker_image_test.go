package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDockerSpawner_Spawn_ImageFlag(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	mockPoller := new(MockPoller)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	imageName := "custom-image:v1.2.3"
	spawner := NewDockerSpawner(logger, mockDocker, imageName, "test-proj", mockPoller, "provider", "model", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
	}

	ctx := context.Background()

	// Channels for capturing data from background goroutine
	execCmdChan := make(chan []string, 1)
	finalSessionChan := make(chan *runner.SessionState, 1)

	// 1. RunContainer
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)

	// 2. Initial SaveSession
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "running"
	})).Return(nil)

	// 3. Exec - Capture arguments instead of strictly matching in goroutine to avoid panic
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Run(func(args mock.Arguments) {
		cmd := args.Get(2).([]string)
		execCmdChan <- cmd
	}).Return("output", nil)

	// 4. LoadSession
	// Return a valid session to allow flow to continue
	mockSM.On("LoadSession", "TICKET-1").Return(&runner.SessionState{
		Name:   "TICKET-1",
		Status: "running",
	}, nil)

	// 5. CurrentCommitSHA (called because flow continues)
	mockGit.On("CurrentCommitSHA", mock.AnythingOfType("string")).Return("sha123", nil)

	// 6. Final SaveSession
	// This signals completion. We check if status is completed or error to capture it.
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "completed" || s.Status == "error"
	})).Run(func(args mock.Arguments) {
		s := args.Get(0).(*runner.SessionState)
		finalSessionChan <- s
	}).Return(nil)

	err := spawner.Spawn(ctx, item)
	assert.NoError(t, err)

	select {
	case finalSession := <-finalSessionChan:
		// Verify final session state
		assert.Equal(t, "completed", finalSession.Status)
		assert.Equal(t, "sha123", finalSession.EndCommitSHA)

		// Verify Exec arguments
		select {
		case cmd := <-execCmdChan:
			if len(cmd) >= 3 {
				cmdStr := cmd[2] // /bin/sh -c <cmdStr>
				assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
				assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
			} else {
				t.Errorf("Exec command too short: %v", cmd)
			}
		default:
			t.Error("Exec was not called but SaveSession was?")
		}

	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for final SaveSession call")
	}

	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
	mockGit.AssertExpectations(t)
}
