package main

import (
	"context"
	"fmt"
	"io"
	"testing"

	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sversion "k8s.io/apimachinery/pkg/version"
)

type MockK8sClient struct {
	ListPodsFunc         func(ctx context.Context, labelSelector string) ([]corev1.Pod, error)
	DeletePodFunc        func(ctx context.Context, name string) error
	GetServerVersionFunc func() (*k8sversion.Info, error)
	GetNamespaceFunc     func() string
	GetPodLogsFunc       func(ctx context.Context, name string, follow bool) (io.ReadCloser, error)
}

func (m *MockK8sClient) ListPods(ctx context.Context, labelSelector string) ([]corev1.Pod, error) {
	if m.ListPodsFunc != nil {
		return m.ListPodsFunc(ctx, labelSelector)
	}
	return nil, nil
}

func (m *MockK8sClient) DeletePod(ctx context.Context, name string) error {
	if m.DeletePodFunc != nil {
		return m.DeletePodFunc(ctx, name)
	}
	return nil
}

func (m *MockK8sClient) GetServerVersion() (*k8sversion.Info, error) {
	if m.GetServerVersionFunc != nil {
		return m.GetServerVersionFunc()
	}
	return &k8sversion.Info{}, nil
}

func (m *MockK8sClient) GetNamespace() string {
	if m.GetNamespaceFunc != nil {
		return m.GetNamespaceFunc()
	}
	return "default"
}

func (m *MockK8sClient) GetPodLogs(ctx context.Context, name string, follow bool) (io.ReadCloser, error) {
	if m.GetPodLogsFunc != nil {
		return m.GetPodLogsFunc(ctx, name, follow)
	}
	return nil, nil
}

func TestStopCmd_LocalSession(t *testing.T) {
	// Setup MockSessionManager
	mockSM := NewMockSessionManager()
	mockSM.Sessions["session1"] = &runner.SessionState{Name: "session1", Status: "running", PID: 123}

	// Override factory
	origSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = origSMFactory }()

	// Execute command
	output, err := executeCommand(rootCmd, "stop", "session1")
	require.NoError(t, err)
	assert.Contains(t, output, "Session 'session1' stopped successfully")
	assert.Equal(t, "stopped", mockSM.Sessions["session1"].Status)
}

func TestStopCmd_K8sSession(t *testing.T) {
	// Setup MockSessionManager (empty)
	mockSM := NewMockSessionManager()

	// Override SM factory
	origSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = origSMFactory }()

	// Setup MockK8sClient
	mockK8s := &MockK8sClient{
		ListPodsFunc: func(ctx context.Context, selector string) ([]corev1.Pod, error) {
			return []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "pod-123",
						Labels: map[string]string{"ticket": "ticket-1"},
					},
				},
			}, nil
		},
		DeletePodFunc: func(ctx context.Context, name string) error {
			if name == "pod-123" {
				return nil
			}
			return fmt.Errorf("pod not found")
		},
	}

	// Override K8s factory
	origK8sFactory := k8sClientFactory
	k8sClientFactory = func() (IK8sClient, error) {
		return mockK8s, nil
	}
	defer func() { k8sClientFactory = origK8sFactory }()

	// Execute command
	output, err := executeCommand(rootCmd, "stop", "ticket-1")
	require.NoError(t, err)
	assert.Contains(t, output, "K8s pod for session 'ticket-1' (pod: pod-123) deleted successfully")
}

func TestStopCmd_NotFound(t *testing.T) {
	// Setup MockSessionManager (empty)
	mockSM := NewMockSessionManager()

	// Override SM factory
	origSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = origSMFactory }()

	// Setup MockK8sClient (empty)
	mockK8s := &MockK8sClient{
		ListPodsFunc: func(ctx context.Context, selector string) ([]corev1.Pod, error) {
			return []corev1.Pod{}, nil
		},
	}

	// Override K8s factory
	origK8sFactory := k8sClientFactory
	k8sClientFactory = func() (IK8sClient, error) {
		return mockK8s, nil
	}
	defer func() { k8sClientFactory = origK8sFactory }()

	// Execute command
	_, err := executeCommand(rootCmd, "stop", "non-existent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}
