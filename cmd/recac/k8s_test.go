package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MockK8sClientTestify struct {
	mock.Mock
}

func (m *MockK8sClientTestify) ListPods(ctx context.Context, labelSelector string) ([]corev1.Pod, error) {
	args := m.Called(ctx, labelSelector)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]corev1.Pod), args.Error(1)
}

func (m *MockK8sClientTestify) DeletePod(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockK8sClientTestify) GetPodLogs(ctx context.Context, name string, tailLines int64) (string, error) {
	args := m.Called(ctx, name, tailLines)
	return args.String(0), args.Error(1)
}

func TestK8sList(t *testing.T) {
	mockClient := new(MockK8sClientTestify)

	// Override factory
	oldFactory := k8sClientFactory
	k8sClientFactory = func() (IK8sClient, error) {
		return mockClient, nil
	}
	defer func() { k8sClientFactory = oldFactory }()

	now := time.Now()
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "recac-agent-rd-123",
				CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Minute)),
				Labels:            map[string]string{"ticket": "RD-123"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
	}

	mockClient.On("ListPods", mock.Anything, "app=recac-agent").Return(pods, nil)

	// Use k8sCmd directly as root for simpler testing
	output, err := executeCommand(rootCmd, "k8s", "list")
	assert.NoError(t, err)
	assert.Contains(t, output, "recac-agent-rd-123")
	assert.Contains(t, output, "Running")
	assert.Contains(t, output, "RD-123")
}

func TestK8sLogs(t *testing.T) {
	mockClient := new(MockK8sClientTestify)
	oldFactory := k8sClientFactory
	k8sClientFactory = func() (IK8sClient, error) {
		return mockClient, nil
	}
	defer func() { k8sClientFactory = oldFactory }()

	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "recac-agent-rd-123",
				Labels: map[string]string{"ticket": "RD-123"},
			},
		},
	}

	// Case 1: By Pod Name
	mockClient.On("ListPods", mock.Anything, "app=recac-agent").Return(pods, nil).Once()
	mockClient.On("GetPodLogs", mock.Anything, "recac-agent-rd-123", int64(100)).Return("log content", nil).Once()

	output, err := executeCommand(rootCmd, "k8s", "logs", "recac-agent-rd-123")
	assert.NoError(t, err)
	assert.Contains(t, output, "log content")

	// Case 2: By Ticket ID
	mockClient.On("ListPods", mock.Anything, "app=recac-agent").Return(pods, nil).Once()
	mockClient.On("GetPodLogs", mock.Anything, "recac-agent-rd-123", int64(100)).Return("log content ticket", nil).Once()

	output, err = executeCommand(rootCmd, "k8s", "logs", "RD-123")
	assert.NoError(t, err)
	assert.Contains(t, output, "log content ticket")
}

func TestK8sCleanup(t *testing.T) {
	mockClient := new(MockK8sClientTestify)
	oldFactory := k8sClientFactory
	k8sClientFactory = func() (IK8sClient, error) {
		return mockClient, nil
	}
	defer func() { k8sClientFactory = oldFactory }()

	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-running"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-succeeded"},
			Status:     corev1.PodStatus{Phase: corev1.PodSucceeded},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-failed"},
			Status:     corev1.PodStatus{Phase: corev1.PodFailed},
		},
	}

	mockClient.On("ListPods", mock.Anything, "app=recac-agent").Return(pods, nil)
	mockClient.On("DeletePod", mock.Anything, "pod-succeeded").Return(nil)
	mockClient.On("DeletePod", mock.Anything, "pod-failed").Return(nil)

	output, err := executeCommand(rootCmd, "k8s", "cleanup")
	assert.NoError(t, err)
	assert.Contains(t, output, "Deleting pod-succeeded (Succeeded)")
	assert.Contains(t, output, "Deleting pod-failed (Failed)")
	assert.Contains(t, output, "Deleted 2 pods")

	mockClient.AssertNotCalled(t, "DeletePod", mock.Anything, "pod-running")
}
