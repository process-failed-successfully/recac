package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestDockerClient wraps MockDockerClient to intercept Exec calls safely
type TestDockerClient struct {
	*MockDockerClient
	execCalled chan string
}

func (m *TestDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	if len(cmd) > 2 {
		select {
		case m.execCalled <- cmd[2]:
		default:
		}
	} else {
		select {
		case m.execCalled <- "":
		default:
		}
	}
	return m.MockDockerClient.Exec(ctx, containerID, cmd)
}

func TestDockerSpawner_Spawn_ImageFlag(t *testing.T) {
	mockDockerBase := new(MockDockerClient)
	execCalled := make(chan string, 1)
	mockDocker := &TestDockerClient{
		MockDockerClient: mockDockerBase,
		execCalled:       execCalled,
	}

	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	mockPoller := new(MockPoller)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	imageName := "custom-image:v1.2.3"
	// Use the wrapped client
	spawner := NewDockerSpawner(logger, mockDocker, imageName, "test-proj", mockPoller, "provider", "model", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
	}

	ctx := context.Background()

	// Mock expectations
	// Use mock.Anything for context to ensure it matches even if wrapped
	mockDockerBase.On("RunContainer", mock.Anything, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)
	mockSM.On("SaveSession", mock.Anything).Return(nil)

	done := make(chan struct{}, 1)
	mockSM.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
		select {
		case done <- struct{}{}:
		default:
		}
	}).Return(nil, assert.AnError)

	failure := make(chan string, 1)

	// Expect the specific container ID "container123" returned by RunContainer
	// No Run hook needed here as TestDockerClient intercepts it
	mockDockerBase.On("Exec", mock.Anything, "container123", mock.Anything).Return("output", nil)

	// If a panic occurs, UpdateStatus will be called with "Failed"
	mockPoller.On("UpdateStatus", mock.Anything, mock.Anything, "Failed", mock.Anything).Run(func(args mock.Arguments) {
		comment := args.String(3)
		failure <- comment
	}).Return(nil).Maybe()

	err := spawner.Spawn(ctx, item)
	require.NoError(t, err)

	select {
	case cmdStr := <-execCalled:
		t.Logf("Captured Command: %s", cmdStr)
		if cmdStr != "" {
			assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
			assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
		} else {
			t.Error("Received empty command string (cmd length <= 2)")
		}
	case failMsg := <-failure:
		t.Fatalf("Spawn failed with error: %s", failMsg)
	case <-time.After(60 * time.Second): // Generous timeout for CI
		t.Fatal("Timeout waiting for Exec call")
	}

	// Wait for background goroutine to finish (LoadSession)
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Log("Timeout waiting for LoadSession (background goroutine cleanup)")
	}
}
