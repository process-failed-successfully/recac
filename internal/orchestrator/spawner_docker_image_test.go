package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"strings"
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

	// Mock expectations
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)
	mockSM.On("SaveSession", mock.Anything).Return(nil)
	mockSM.On("LoadSession", "TICKET-1").Return(nil, assert.AnError)

	failure := make(chan string, 1)
	mockPoller.On("UpdateStatus", mock.Anything, mock.Anything, "Failed", mock.Anything).Run(func(args mock.Arguments) {
		comment := args.String(3)
		failure <- comment
	}).Return(nil).Maybe()

	done := make(chan struct{})

	// We match arguments using MatchedBy, then signal completion via done channel
	mockDocker.On("Exec", mock.Anything, "container123", mock.MatchedBy(func(cmd []string) bool {
		if len(cmd) < 3 {
			return false
		}
		cmdStr := cmd[2]
		// Check for --image flag and correct image name
		return strings.Contains(cmdStr, "--image") && strings.Contains(cmdStr, imageName)
	})).Run(func(args mock.Arguments) {
		close(done)
	}).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	assert.NoError(t, err)

	select {
	case <-done:
		// Success
	case msg := <-failure:
		t.Fatalf("Background task failed: %s", msg)
	case <-time.After(60 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}
}
