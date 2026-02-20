package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"recac/internal/runner"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
)

func TestDockerSpawner_PlanOnly(t *testing.T) {
	mockClient := new(MockDockerClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockSM := new(MockSessionManager)

	spawner := NewDockerSpawner(logger, mockClient, "image", "project", nil, "provider", "model", mockSM)

	// Test Case 1: PlanOnly = true
	item := WorkItem{
		ID:       "TEST-1",
		RepoURL:  "https://github.com/org/repo",
		PlanOnly: true,
	}

	var capturedCmd []string
	// Mock RunContainerWithLabels to capture command
	mockClient.On("RunContainerWithLabels", mock.Anything, "image", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedCmd = args.Get(5).([]string) // cmd is 5th argument (0-indexed? check definition)
		// Definition: RunContainerWithLabels(ctx, image, workspace, binds, env, cmd, user, labels)
		// indices: 0, 1, 2, 3, 4, 5, 6, 7
		// Yes, cmd is 5.
	}).Return("mock-container-id", nil).Once()

	mockClient.On("WaitContainer", mock.Anything, "mock-container-id").Return(int(0), nil).Once()

	mockSM.On("SaveSession", mock.AnythingOfType("*runner.SessionState")).Return(nil).Once()
	mockSM.On("LoadSession", "TEST-1").Return(&runner.SessionState{}, nil).Once()
	mockSM.On("SaveSession", mock.AnythingOfType("*runner.SessionState")).Return(nil).Once()

	if err := spawner.Spawn(context.Background(), item); err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Verify command contains --plan
	cmdStr := strings.Join(capturedCmd, " ")
	if !strings.Contains(cmdStr, "--plan") {
		t.Errorf("Expected command to contain --plan, got: %s", cmdStr)
	}

	// Test Case 2: PlanOnly = false
	item.PlanOnly = false

	// Reset mocks? or create new ones? New ones are safer.
	mockClient2 := new(MockDockerClient)
	mockSM2 := new(MockSessionManager)
	spawner2 := NewDockerSpawner(logger, mockClient2, "image", "project", nil, "provider", "model", mockSM2)

	mockClient2.On("RunContainerWithLabels", mock.Anything, "image", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedCmd = args.Get(5).([]string)
	}).Return("mock-container-id-2", nil).Once()

	mockClient2.On("WaitContainer", mock.Anything, "mock-container-id-2").Return(int(0), nil).Once()

	mockSM2.On("SaveSession", mock.AnythingOfType("*runner.SessionState")).Return(nil).Once()
	mockSM2.On("LoadSession", "TEST-1").Return(&runner.SessionState{}, nil).Once()
	mockSM2.On("SaveSession", mock.AnythingOfType("*runner.SessionState")).Return(nil).Once()

	if err := spawner2.Spawn(context.Background(), item); err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	cmdStr = strings.Join(capturedCmd, " ")
	if strings.Contains(cmdStr, "--plan") {
		t.Errorf("Expected command NOT to contain --plan, got: %s", cmdStr)
	}
}
