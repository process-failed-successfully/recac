package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// setupLogsTest configures a mock session manager with temporary log files.
func setupLogsTest(t *testing.T) (*MockSessionManager, func()) {
	t.Helper()

	mockSM := NewMockSessionManager()

	// Create a temp dir for log files
	tmpDir, err := os.MkdirTemp("", "recac-logs-test-")
	require.NoError(t, err)

	// --- Create mock sessions and their log files ---
	session1Log := filepath.Join(tmpDir, "session1.log")
	err = os.WriteFile(session1Log, []byte("session 1 log line 1\nsession 1 log line 2\n"), 0644)
	require.NoError(t, err)

	session2Log := filepath.Join(tmpDir, "session2.log")
	err = os.WriteFile(session2Log, []byte("session 2 log line 1\n"), 0644)
	require.NoError(t, err)

	stoppedSessionLog := filepath.Join(tmpDir, "stopped.log")
	err = os.WriteFile(stoppedSessionLog, []byte("this should not be read\n"), 0644)
	require.NoError(t, err)

	mockSM.Sessions = map[string]*runner.SessionState{
		"session1": {
			Name:    "session1",
			Status:  "running",
			LogFile: session1Log,
		},
		"session2": {
			Name:    "session2",
			Status:  "running",
			LogFile: session2Log,
		},
		"stopped-session": {
			Name:    "stopped-session",
			Status:  "stopped",
			LogFile: stoppedSessionLog,
		},
	}

	// Monkey-patch the sessionManagerFactory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}

	// Also patch k8sClientFactory to return nil by default to match original behavior
	origK8sFactory := k8sClientFactory
	k8sClientFactory = func() (IK8sClient, error) {
		return nil, nil
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
		sessionManagerFactory = originalFactory
		k8sClientFactory = origK8sFactory
	}

	return mockSM, cleanup
}

func setupLogsTestWithK8s(t *testing.T) (*MockSessionManager, *MockK8sClient, func()) {
	t.Helper()

	mockSM, cleanupSM := setupLogsTest(t)

	mockK8s := &MockK8sClient{}

	k8sClientFactory = func() (IK8sClient, error) {
		return mockK8s, nil
	}

	cleanup := func() {
		cleanupSM() // This restores factories, so we rely on it or override back manually if needed.
		// Actually setupLogsTest cleans up sessionManagerFactory and k8sClientFactory (to nil).
		// But we just overwrote k8sClientFactory.
		// So we don't need to do anything else for k8sClientFactory as cleanupSM will restore the ORIGINAL one saved in setupLogsTest.
		// Wait, setupLogsTest saves `origK8sFactory`.
	}

	return mockSM, mockK8s, cleanup
}

func TestLogsCmd(t *testing.T) {
	t.Run("logs --all streams running sessions", func(t *testing.T) {
		_, cleanup := setupLogsTest(t)
		defer cleanup()

		output, err := executeCommand(rootCmd, "logs", "--all")
		require.NoError(t, err)

		// Check that output from both running sessions is present
		assert.Contains(t, output, "[session1] session 1 log line 1")
		assert.Contains(t, output, "[session1] session 1 log line 2")
		assert.Contains(t, output, "[session2] session 2 log line 1")

		// Check that output from the stopped session is not present
		assert.NotContains(t, output, "stopped-session")
		assert.NotContains(t, output, "this should not be read")
	})

	t.Run("logs single session", func(t *testing.T) {
		_, cleanup := setupLogsTest(t)
		defer cleanup()

		output, err := executeCommand(rootCmd, "logs", "session1")
		require.NoError(t, err)

		assert.Contains(t, output, "session 1 log line 1")
		assert.Contains(t, output, "session 1 log line 2")
		assert.NotContains(t, output, "[session1]") // No prefix for single log
		assert.NotContains(t, output, "session 2")
	})

	t.Run("logs --all with filter", func(t *testing.T) {
		_, cleanup := setupLogsTest(t)
		defer cleanup()

		output, err := executeCommand(rootCmd, "logs", "--all", "--filter", "line 2")
		require.NoError(t, err)

		assert.Contains(t, output, "[session1] session 1 log line 2")
		assert.NotContains(t, output, "session 1 log line 1")
		assert.NotContains(t, output, "session 2")
	})

	t.Run("logs --all with no running sessions", func(t *testing.T) {
		mockSM, cleanup := setupLogsTest(t)
		defer cleanup()

		// Override to have no running sessions
		mockSM.Sessions["session1"].Status = "completed"
		mockSM.Sessions["session2"].Status = "error"

		output, err := executeCommand(rootCmd, "logs", "--all")
		require.NoError(t, err)

		assert.Contains(t, output, "No running sessions found.")
	})

	t.Run("logs validation errors", func(t *testing.T) {
		_, cleanup := setupLogsTest(t)
		defer cleanup()

		_, err := executeCommand(rootCmd, "logs")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires a session name or --all flag")

		_, err = executeCommand(rootCmd, "logs", "session1", "--all")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot use session name with --all")

		_, err = executeCommand(rootCmd, "logs", "non-existent-session")
		// This will print an error and exit(1), which is caught by executeCommand
		// and does not return a Go error. We check stderr.
		output, _ := executeCommand(rootCmd, "logs", "non-existent-session")
		assert.Contains(t, output, "Error: session 'non-existent-session' not found locally or in Kubernetes")
	})

	t.Run("logs from k8s pod", func(t *testing.T) {
		_, mockK8s, cleanup := setupLogsTestWithK8s(t)
		defer cleanup()

		podName := "pod-ticket-123"
		ticketID := "ticket-123"
		logContent := "k8s log line 1\nk8s log line 2\n"

		mockK8s.ListPodsFunc = func(ctx context.Context, selector string) ([]corev1.Pod, error) {
			if selector == fmt.Sprintf("ticket=%s", ticketID) {
				return []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: podName,
							Labels: map[string]string{
								"ticket": ticketID,
							},
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
						},
					},
				}, nil
			}
			return nil, nil
		}

		mockK8s.GetPodLogsFunc = func(ctx context.Context, name string, follow bool) (io.ReadCloser, error) {
			if name == podName {
				return io.NopCloser(bytes.NewBufferString(logContent)), nil
			}
			return nil, fmt.Errorf("pod not found")
		}

		output, err := executeCommand(rootCmd, "logs", ticketID)
		require.NoError(t, err)
		assert.Contains(t, output, "k8s log line 1")
		assert.Contains(t, output, "k8s log line 2")
	})

	t.Run("logs --all includes k8s pods", func(t *testing.T) {
		_, mockK8s, cleanup := setupLogsTestWithK8s(t)
		defer cleanup()

		podName := "pod-ticket-123"
		ticketID := "ticket-123"
		logContent := "k8s log line 1\n"

		mockK8s.ListPodsFunc = func(ctx context.Context, selector string) ([]corev1.Pod, error) {
			if selector == "ticket" {
				return []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: podName,
							Labels: map[string]string{
								"ticket": ticketID,
							},
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
						},
					},
				}, nil
			}
			return nil, nil
		}

		mockK8s.GetPodLogsFunc = func(ctx context.Context, name string, follow bool) (io.ReadCloser, error) {
			if name == podName {
				return io.NopCloser(bytes.NewBufferString(logContent)), nil
			}
			return nil, fmt.Errorf("pod not found")
		}

		output, err := executeCommand(rootCmd, "logs", "--all")
		require.NoError(t, err)
		assert.Contains(t, output, "[session1] session 1 log line 1")
		assert.Contains(t, output, fmt.Sprintf("[%s] k8s log line 1", ticketID))
	})
}
