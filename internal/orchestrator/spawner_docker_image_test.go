package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"recac/internal/runner"
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

	// Verify initial SaveSession captures correct command in SessionState
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		// Check if command contains --image
		found := false
		for i, arg := range s.Command {
			if arg == "--image" && i+1 < len(s.Command) && s.Command[i+1] == imageName {
				found = true
				break
			}
		}
		return found
	})).Return(nil)

	done := make(chan struct{}, 1)
	mockSM.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
		select {
		case done <- struct{}{}:
		default:
		}
	}).Return(nil, assert.AnError)

	execCalled := make(chan string, 10) // Buffer size > 1 just in case
	failure := make(chan string, 10)

	// Expect Exec call. Match containerID strictly.
	// Arguments: ctx, containerID, cmd
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Run(func(args mock.Arguments) {
		defer func() {
			if r := recover(); r != nil {
				failure <- fmt.Sprintf("Panic in Exec mock: %v", r)
			}
		}()

		cmdRaw := args.Get(2)
		cmd, ok := cmdRaw.([]string)
		if !ok {
			failure <- fmt.Sprintf("Exec called with non-[]string command: %T", cmdRaw)
			return
		}

		if len(cmd) > 2 {
			execCalled <- cmd[2]
		} else {
			execCalled <- ""
		}
	}).Return("output", nil)

	// UpdateStatus mock to catch failures. Match ANY arguments.
	// Arguments: ctx, item, status, comment
	mockPoller.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		defer func() {
			if r := recover(); r != nil {
				failure <- fmt.Sprintf("Panic in UpdateStatus mock: %v", r)
			}
		}()

		statusRaw := args.Get(2)
		status, ok := statusRaw.(string)
		if !ok {
			failure <- fmt.Sprintf("UpdateStatus called with non-string status: %T", statusRaw)
			return
		}

		commentRaw := args.Get(3)
		comment, ok := commentRaw.(string)
		if !ok {
			failure <- fmt.Sprintf("UpdateStatus called with non-string comment: %T", commentRaw)
			return
		}

		failure <- fmt.Sprintf("Status: %s, Comment: %s", status, comment)
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
		t.Fatalf("Spawn failed or panicked: %s", failMsg)
	case <-time.After(60 * time.Second):
		t.Fatal("Timeout waiting for Exec call (possible goroutine hang or panic)")
	}

	// Wait for background goroutine to finish (LoadSession)
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Log("Timeout waiting for LoadSession (background goroutine cleanup)")
	}

	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
}
