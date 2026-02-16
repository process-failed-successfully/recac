package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
	// Use mock.Anything for context to ensure it matches even if wrapped
	mockDocker.On("RunContainer", mock.Anything, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)
	mockSM.On("SaveSession", mock.Anything).Return(nil)

	done := make(chan struct{}, 1)
	failure := make(chan string, 1)

	// LoadSession mock to signal completion
	mockSM.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
		select {
		case done <- struct{}{}:
		default:
		}
	}).Return(nil, assert.AnError)

	// UpdateStatus mock to catch failures
	mockPoller.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		statusRaw := args.Get(2)
		status, _ := statusRaw.(string)
		commentRaw := args.Get(3)
		comment, _ := commentRaw.(string)
		failure <- fmt.Sprintf("Status: %s, Comment: %s", status, comment)
	}).Return(nil).Maybe()

	// Expect Exec call. Use MatchedBy for robust argument verification.
	mockDocker.On("Exec", mock.Anything, "container123", mock.MatchedBy(func(cmd []string) bool {
		if len(cmd) < 3 {
			return false
		}
		cmdStr := cmd[2]
		// Check for --image flag and the correct image name
		return strings.Contains(cmdStr, "--image") && strings.Contains(cmdStr, imageName)
	})).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	select {
	case <-done:
		// Success
	case failMsg := <-failure:
		t.Fatalf("Spawn failed or panicked: %s", failMsg)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for background task completion")
	}
}
