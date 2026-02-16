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
	spawner := NewDockerSpawner(logger, mockDocker, imageName, "test-proj", mockPoller, "provider", "model", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
	}

	ctx := context.Background()

	// Mock expectations
	// verify that binds contains docker socket
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.MatchedBy(func(binds []string) bool {
		for _, b := range binds {
			if b == "/var/run/docker.sock:/var/run/docker.sock" {
				return true
			}
		}
		return false
	}), mock.Anything, "").Return("container123", nil).Once()
	mockSM.On("SaveSession", mock.Anything).Return(nil).Once()

	// Use a channel to ensure the goroutine calls LoadSession before we exit
	loadSessionCalled := make(chan struct{}, 1)
	mockSM.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
		select {
		case loadSessionCalled <- struct{}{}:
		default:
			// Prevent blocking if called multiple times (though Once() should prevent that)
		}
	}).Return(nil, assert.AnError).Once()

	type execCall struct {
		containerID string
		cmd         []string
	}
	execCalled := make(chan execCall, 1)

	// We match "Anything" for arguments so we catch the call, then inspect it in Run
	// This prevents the mock from panicking in the background goroutine if arguments mismatch,
	// which would cause the test to timeout waiting for the signal.
	mockDocker.On("Exec", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		cid := args.String(1)
		cmd := args.Get(2).([]string)
		// cmd is ["/bin/sh", "-c", "actual command"]
		if len(cmd) > 2 {
			select {
			case execCalled <- execCall{containerID: cid, cmd: cmd}:
			default:
			}
		}
	}).Return("output", nil).Once()

	err := spawner.Spawn(ctx, item)
	assert.NoError(t, err)

	select {
	case call := <-execCalled:
		assert.Equal(t, "container123", call.containerID, "Exec called with wrong container ID")
		cmdStr := call.cmd[2]
		t.Logf("Captured Command: %s", cmdStr)
		assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(60 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}

	// Wait for the goroutine to finish its critical section (calling LoadSession)
	select {
	case <-loadSessionCalled:
		// Success: LoadSession was called
	case <-time.After(60 * time.Second):
		t.Fatal("Timeout waiting for LoadSession call")
	}
}
