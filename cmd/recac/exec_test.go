package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"recac/internal/docker"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// --- Mocks ---

type MockDockerClient struct {
	ExecInteractiveFunc func(ctx context.Context, containerID string, cmd []string) error
}

func (m *MockDockerClient) ExecInteractive(ctx context.Context, containerID string, cmd []string) error {
	if m.ExecInteractiveFunc != nil {
		return m.ExecInteractiveFunc(ctx, containerID, cmd)
	}
	return nil
}

type MockK8sClientForExec struct {
	ListPodsFunc  func(ctx context.Context, labelSelector string) ([]corev1.Pod, error)
	DeletePodFunc func(ctx context.Context, name string) error
}

func (m *MockK8sClientForExec) ListPods(ctx context.Context, labelSelector string) ([]corev1.Pod, error) {
	if m.ListPodsFunc != nil {
		return m.ListPodsFunc(ctx, labelSelector)
	}
	return nil, nil
}

func (m *MockK8sClientForExec) DeletePod(ctx context.Context, name string) error {
	if m.DeletePodFunc != nil {
		return m.DeletePodFunc(ctx, name)
	}
	return nil
}

// --- Helper Process for os/exec Mocking ---

func TestExecCmdHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	// Parse args to find the actual command
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd := args[0]
	// cmdArgs := args[1:]

	switch cmd {
	case "kubectl":
		fmt.Fprintf(os.Stdout, "kubectl execution mocked")
		os.Exit(0)
	case "ls":
		fmt.Fprintf(os.Stdout, "local execution mocked")
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s", cmd)
		os.Exit(1)
	}
}

// --- Tests ---

func TestExecCmd_Docker(t *testing.T) {
	// 1. Setup mocks
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	mockDocker := &MockDockerClient{}
	dockerFactory = func(project string) (DockerExecClient, error) {
		return mockDocker, nil
	}
	defer func() {
		dockerFactory = func(project string) (DockerExecClient, error) {
			return docker.NewClient(project)
		}
	}()

	// 2. Setup a mock session
	sessionName := "test-docker-session"
	sessionState := &runner.SessionState{
		Name:        sessionName,
		Status:      "running",
		ContainerID: "test-container-id",
		PID:         os.Getpid(),
		StartTime:   time.Now(),
	}
	require.NoError(t, sm.SaveSession(sessionState))

	// 3. Setup expectations
	var execInteractiveCalled bool
	mockDocker.ExecInteractiveFunc = func(ctx context.Context, containerID string, cmd []string) error {
		execInteractiveCalled = true
		assert.Equal(t, "test-container-id", containerID)
		assert.Equal(t, []string{"ls", "-la"}, cmd)
		return nil
	}

	// 4. Run the command
	// We use the real RunE, no overriding
	output, err := executeCommand(rootCmd, "exec", sessionName, "--", "ls", "-la")
	require.NoError(t, err)
	_ = output // check if needed

	assert.True(t, execInteractiveCalled, "ExecInteractive should have been called")
}

func TestExecCmd_Local(t *testing.T) {
	// 1. Setup mocks
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	// Mock execCommand
	oldExecCommand := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestExecCmdHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}
	defer func() { execCommand = oldExecCommand }()

	// 2. Setup a mock session
	sessionName := "test-local-session"
	tmpDir := t.TempDir()
	sessionState := &runner.SessionState{
		Name:        sessionName,
		Status:      "running",
		ContainerID: "local", // Trigger local path
		PID:         os.Getpid(),
		StartTime:   time.Now(),
		Workspace:   tmpDir,
	}
	require.NoError(t, sm.SaveSession(sessionState))

	// 3. Run the command
	output, err := executeCommand(rootCmd, "exec", sessionName, "--", "ls", "-la")
	require.NoError(t, err)

	// 4. Assertions
	assert.Contains(t, output, "Executing locally in workspace")
	assert.Contains(t, output, "local execution mocked")
}

func TestExecCmd_K8s(t *testing.T) {
	// 1. Setup mocks
	_, cleanup := setupTestSessionManager(t)
	defer cleanup()

	// Mock execCommand for kubectl
	oldExecCommand := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestExecCmdHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}
	defer func() { execCommand = oldExecCommand }()

	// Mock K8s Client
	mockK8s := &MockK8sClientForExec{}
	k8sClientFactory = func() (IK8sClient, error) {
		return mockK8s, nil
	}
	// Restore factory
	oldK8sFactory := k8sClientFactory
	defer func() { k8sClientFactory = oldK8sFactory }()

	// 2. Setup expectations
	mockK8s.ListPodsFunc = func(ctx context.Context, labelSelector string) ([]corev1.Pod, error) {
		assert.Contains(t, labelSelector, "ticket=test-k8s-session")
		return []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-123",
					Namespace: "test-ns",
				},
			},
		}, nil
	}

	// 3. Run the command (Session NOT in SM)
	output, err := executeCommand(rootCmd, "exec", "test-k8s-session", "--", "ls", "-la")
	require.NoError(t, err)

	// 4. Assertions
	assert.Contains(t, output, "Found K8s pod: test-pod-123")
	assert.Contains(t, output, "kubectl execution mocked")
}
