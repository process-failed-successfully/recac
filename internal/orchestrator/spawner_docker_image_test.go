package orchestrator

import (
	"context"
	"io"
	"log/slog"
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
	spawner := NewDockerSpawner(logger, mockDocker, imageName, "test-proj", mockPoller, "provider", "model", "Always", mockSM, 30, 5, 10)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
	}

	ctx := context.Background()

	// Mock expectations
	mockDocker.On("PullImage", ctx, imageName).Return(nil)

	// We match "Anything" for arguments so we catch the call, then inspect it in Run
	runCalled := make(chan string, 1)
	mockDocker.On("RunContainerWithLabels", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, "", mock.Anything).Run(func(args mock.Arguments) {
		cmd := args.Get(5).([]string) // env=4, cmd=5
		// cmd is ["/bin/sh", "-c", "actual command"]
		runCalled <- cmd[2]
	}).Return("container123", nil)

	mockDocker.On("WaitContainer", ctx, "container123").Return(int(0), nil)

	mockSM.On("SaveSession", mock.Anything).Return(nil)
	mockSM.On("LoadSession", "TICKET-1").Return(nil, assert.AnError)

	err := spawner.Spawn(ctx, item)
	assert.NoError(t, err)

	select {
	case cmdStr := <-runCalled:
		t.Logf("Captured Command: %s", cmdStr)
		assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for RunContainerWithLabels call")
	}
}
