package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"recac/internal/runner"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockLogK8sClient is a mock implementation of IK8sClient
type MockLogK8sClient struct {
	Pods []corev1.Pod
	Logs map[string]string
}

func (m *MockLogK8sClient) ListPods(ctx context.Context, labelSelector string) ([]corev1.Pod, error) {
	if strings.Contains(labelSelector, "ticket=") {
		ticket := strings.TrimPrefix(labelSelector, "ticket=")
		var res []corev1.Pod
		for _, p := range m.Pods {
			if p.Labels["ticket"] == ticket {
				res = append(res, p)
			}
		}
		return res, nil
	}
	// Basic filtering for "app=recac-agent" if needed, but for now return all if no specific ticket
	return m.Pods, nil
}

func (m *MockLogK8sClient) GetPodLogs(ctx context.Context, name string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	if content, ok := m.Logs[name]; ok {
		return io.NopCloser(strings.NewReader(content)), nil
	}
	return nil, fmt.Errorf("pod not found")
}

func (m *MockLogK8sClient) DeletePod(ctx context.Context, name string) error {
	return nil
}

func TestLogsCmd_Local(t *testing.T) {
	// Setup Mocks
	mockSM := NewMockSessionManager()

	// Create a temp log file
	tmpFile, err := os.CreateTemp("", "recac-log-test-")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.WriteString("local log content\nline 2\n")
	assert.NoError(t, err)
	tmpFile.Close()

	mockSM.Sessions["session1"] = &runner.SessionState{
		Name:    "session1",
		Status:  "running",
		LogFile: tmpFile.Name(),
	}

	mockK8s := &MockLogK8sClient{}

	// Override factories
	oldSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
	defer func() { sessionManagerFactory = oldSMFactory }()

	oldK8sFactory := k8sClientFactory
	k8sClientFactory = func() (IK8sClient, error) { return mockK8s, nil }
	defer func() { k8sClientFactory = oldK8sFactory }()

	// Execute
	output, err := executeCommand(rootCmd, "logs", "session1")

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, output, "local log content")
	assert.Contains(t, output, "line 2")
}

func TestLogsCmd_Remote(t *testing.T) {
	// Setup Mocks
	mockSM := NewMockSessionManager()
	// Ensure GetSessionLogs fails so it falls back to K8s
	// MockSessionManager GetSessionLogs checks m.Sessions. Default is empty.
	// So it should fail with "session not found".

	mockK8s := &MockLogK8sClient{
		Pods: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-123",
					Namespace: "default",
					Labels:    map[string]string{"ticket": "TICKET-123"},
					CreationTimestamp: metav1.Now(),
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
		},
		Logs: map[string]string{
			"pod-123": "remote log content\nline 2\n",
		},
	}

	// Override factories
	oldSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
	defer func() { sessionManagerFactory = oldSMFactory }()

	oldK8sFactory := k8sClientFactory
	k8sClientFactory = func() (IK8sClient, error) { return mockK8s, nil }
	defer func() { k8sClientFactory = oldK8sFactory }()

	// Execute
	output, err := executeCommand(rootCmd, "logs", "TICKET-123")

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, output, "remote log content")
	assert.Contains(t, output, "line 2")
}

func TestLogsCmd_All_Remote(t *testing.T) {
	// Setup Mocks
	mockSM := NewMockSessionManager()

	mockK8s := &MockLogK8sClient{
		Pods: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-123",
					Namespace: "default",
					Labels:    map[string]string{"ticket": "TICKET-123"},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
		},
		Logs: map[string]string{
			"pod-123": "remote log from all",
		},
	}

	// Override factories
	oldSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
	defer func() { sessionManagerFactory = oldSMFactory }()

	oldK8sFactory := k8sClientFactory
	k8sClientFactory = func() (IK8sClient, error) { return mockK8s, nil }
	defer func() { k8sClientFactory = oldK8sFactory }()

	// Execute
	output, err := executeCommand(rootCmd, "logs", "--all")

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, output, "[TICKET-123] remote log from all")
}
