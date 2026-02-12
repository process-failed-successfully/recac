package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockKubectlClient for testing
type MockKubectlClient struct {
	ExecInteractiveFunc func(ctx context.Context, podName string, cmd []string) error
	FindPodFunc         func(ctx context.Context, sessionName string) (string, error)
}

func (m *MockKubectlClient) ExecInteractive(ctx context.Context, podName string, cmd []string) error {
	if m.ExecInteractiveFunc != nil {
		return m.ExecInteractiveFunc(ctx, podName, cmd)
	}
	return nil
}

func (m *MockKubectlClient) FindPod(ctx context.Context, sessionName string) (string, error) {
	if m.FindPodFunc != nil {
		return m.FindPodFunc(ctx, sessionName)
	}
	return "", fmt.Errorf("pod not found")
}

func TestExecCmd_K8s_LocalSession(t *testing.T) {
	// 1. Setup mocks
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	mockKubectl := &MockKubectlClient{}
	kubectlFactory = func() (KubectlExecClient, error) {
		return mockKubectl, nil
	}
	defer func() {
		kubectlFactory = func() (KubectlExecClient, error) {
			return &DefaultKubectlClient{}, nil
		}
	}()

	// 2. Setup a mock session with "local" container ID (simulating K8s session)
	sessionName := "test-k8s-session"
	sessionState := &runner.SessionState{
		Name:        sessionName,
		Status:      "running",
		ContainerID: "local",
		PID:         os.Getpid(),
		StartTime:   time.Now(),
	}
	require.NoError(t, sm.SaveSession(sessionState))

	// 3. Expectations
	mockKubectl.FindPodFunc = func(ctx context.Context, name string) (string, error) {
		assert.Equal(t, sessionName, name)
		return "test-pod-123", nil
	}

	execInteractiveCalled := false
	mockKubectl.ExecInteractiveFunc = func(ctx context.Context, podName string, cmd []string) error {
		execInteractiveCalled = true
		assert.Equal(t, "test-pod-123", podName)
		assert.Equal(t, []string{"ls"}, cmd)
		return nil
	}

	// 4. Run command
	_, err := executeCommand(rootCmd, "exec", sessionName, "--", "ls")
	require.NoError(t, err)

	assert.True(t, execInteractiveCalled, "ExecInteractive should have been called")
}

func TestExecCmd_K8s_NoSession(t *testing.T) {
	// 1. Setup mocks
	_, cleanup := setupTestSessionManager(t)
	defer cleanup()

	mockKubectl := &MockKubectlClient{}
	kubectlFactory = func() (KubectlExecClient, error) {
		return mockKubectl, nil
	}
	defer func() {
		kubectlFactory = func() (KubectlExecClient, error) {
			return &DefaultKubectlClient{}, nil
		}
	}()

	sessionName := "non-existent-session"

	// 3. Expectations
	mockKubectl.FindPodFunc = func(ctx context.Context, name string) (string, error) {
		assert.Equal(t, sessionName, name)
		return "test-pod-456", nil
	}

	execInteractiveCalled := false
	mockKubectl.ExecInteractiveFunc = func(ctx context.Context, podName string, cmd []string) error {
		execInteractiveCalled = true
		assert.Equal(t, "test-pod-456", podName)
		return nil
	}

	// 4. Run command
	_, err := executeCommand(rootCmd, "exec", sessionName, "--", "ls")
	require.NoError(t, err)

	assert.True(t, execInteractiveCalled, "ExecInteractive should have been called even if local session not found")
}

func TestExecCmd_K8s_Fail(t *testing.T) {
	// 1. Setup mocks
	_, cleanup := setupTestSessionManager(t)
	defer cleanup()

	mockKubectl := &MockKubectlClient{}
	kubectlFactory = func() (KubectlExecClient, error) {
		return mockKubectl, nil
	}
	defer func() {
		kubectlFactory = func() (KubectlExecClient, error) {
			return &DefaultKubectlClient{}, nil
		}
	}()

	sessionName := "missing-session"

	// 3. Expectations
	mockKubectl.FindPodFunc = func(ctx context.Context, name string) (string, error) {
		return "", fmt.Errorf("pod not found")
	}

	// 4. Run command - should fail (print error to stderr, but command itself might return nil error because Run handles it)
	// executeCommand returns output and error. If Run returns error, we catch it.
	// But Run in execCmd doesn't return error, it prints to stderr.
	// So we expect no error from executeCommand, but stderr output.
	// However, executeCommand helper in tests/ usually captures stdout/stderr.
	// We verify that it doesn't panic.

	output, err := executeCommand(rootCmd, "exec", sessionName, "--", "ls")
	require.NoError(t, err) // Cobra command didn't return error
	assert.Contains(t, output, "not found locally or in Kubernetes")
}
