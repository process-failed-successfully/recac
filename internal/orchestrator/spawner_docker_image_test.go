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

// TestDockerClient wraps MockDockerClient to intercept Exec calls
// and signal the test even if the mock expectation fails (panics).
type TestDockerClient struct {
	*MockDockerClient
	execChan chan string
}

func (m *TestDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	if len(cmd) > 2 {
		m.execChan <- cmd[2]
	} else {
		m.execChan <- ""
	}
	// Call the original mock to record the call and check expectations
	return m.MockDockerClient.Exec(ctx, containerID, cmd)
}

func TestDockerSpawner_Spawn_ImageFlag(t *testing.T) {
	mockDockerBase := new(MockDockerClient)
	execCalled := make(chan string, 1)

	// Use the wrapper
	mockDocker := &TestDockerClient{
		MockDockerClient: mockDockerBase,
		execChan:         execCalled,
	}

	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	mockPoller := new(MockPoller)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	imageName := "custom-image:v1.2.3"

	// Pass the wrapper (mockDocker) as the client
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
	// We verify arguments but rely on the wrapper for signaling.
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
	case <-time.After(60 * time.Second): // Extended timeout for CI runner latency
		t.Fatal("Timeout waiting for Exec call")
	}

	// Wait for background goroutine to finish (LoadSession)
	// We also listen for failure here, in case Exec panicked after signaling but before LoadSession.
	select {
	case <-done:
		// Success
	case failMsg := <-failure:
		t.Fatalf("Spawn failed (during LoadSession wait) with error: %s", failMsg)
	case <-time.After(10 * time.Second):
		t.Log("Timeout waiting for LoadSession (background goroutine cleanup)")
	}
}
