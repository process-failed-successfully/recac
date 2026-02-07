package main

import (
	"context"
	"testing"
	"time"

	"recac/internal/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*runner.SessionState), args.Error(1)
}

func (m *MockSessionManager) GetSessionLogContent(name string, lines int) (string, error) {
	args := m.Called(name, lines)
	return args.String(0), args.Error(1)
}

type MockK8sClient struct {
	mock.Mock
}

func (m *MockK8sClient) ListPods(ctx context.Context, labelSelector string) ([]corev1.Pod, error) {
	args := m.Called(ctx, labelSelector)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]corev1.Pod), args.Error(1)
}

func TestGetUnifiedSessions(t *testing.T) {
	// Setup mocks
	mockSM := new(MockSessionManager)
	mockK8s := new(MockK8sClient)

	// Override factories
	oldSMFactory := sessionManagerFactory
	oldK8sFactory := k8sClientFactory
	defer func() {
		sessionManagerFactory = oldSMFactory
		k8sClientFactory = oldK8sFactory
	}()

	sessionManagerFactory = func() (SessionLister, error) {
		return mockSM, nil
	}
	k8sClientFactory = func() (PodLister, error) {
		return mockK8s, nil
	}

	// Mock Data
	now := time.Now()
	localSessions := []*runner.SessionState{
		{
			Name:      "local-session-1",
			Status:    "running",
			StartTime: now.Add(-10 * time.Minute),
			PID:       1234, // Mock PID
			// AgentStateFile will fail to load, handling gracefully
		},
	}

	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "remote-pod-1",
				CreationTimestamp: metav1.Time{Time: now.Add(-5 * time.Minute)},
				Labels: map[string]string{"ticket": "JIRA-123"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
	}

	mockSM.On("ListSessions").Return(localSessions, nil)
	mockK8s.On("ListPods", mock.Anything, "app=recac-agent").Return(pods, nil)

	// Execute with Options
	opts := DashboardOptions{
		Remote: true,
	}

	sessions, err := getUnifiedSessions(opts)

	// Assertions
	assert.NoError(t, err)
	assert.Len(t, sessions, 2)

	// Check sorting (newest first)
	// remote-pod-1 is 5 min ago. local-session-1 is 10 min ago.
	// So remote-pod-1 should be first.
	assert.Equal(t, "remote-pod-1", sessions[0].Name)
	assert.Equal(t, "local-session-1", sessions[1].Name)

	assert.Equal(t, "local", sessions[1].Location)
	assert.Equal(t, "k8s", sessions[0].Location)
}
