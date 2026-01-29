package main

import (
	"context"
	"recac/internal/model"
	"recac/internal/runner"
	"recac/internal/ui"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonitorCmd_Structure(t *testing.T) {
	assert.Equal(t, "monitor", monitorCmd.Use)
	assert.NotNil(t, monitorCmd.RunE)
}

func TestMonitorCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "monitor" {
			found = true
			break
		}
	}
	assert.True(t, found, "monitor command should be registered in rootCmd")
}

func TestMonitorCmd_K8sIntegration(t *testing.T) {
	// 1. Mock Session Manager
	mockSM := NewMockSessionManager()
	origSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = origSMFactory }()

	// 2. Mock K8s Client
	mockK8s := &MockK8sClient{
		GetPodLogsFunc: func(ctx context.Context, name string, lines int) (string, error) {
			if name == "pod-123" {
				return "k8s logs content", nil
			}
			return "", nil
		},
		DeletePodFunc: func(ctx context.Context, name string) error {
			if name == "pod-123" {
				return nil
			}
			return nil
		},
	}
	origK8sFactory := k8sClientFactory
	k8sClientFactory = func() (IK8sClient, error) {
		return mockK8s, nil
	}
	defer func() { k8sClientFactory = origK8sFactory }()

	// 3. Mock UI Starter
	var capturedCallbacks ui.ActionCallbacks
	origStartFunc := startMonitorDashboardFunc
	startMonitorDashboardFunc = func(callbacks ui.ActionCallbacks) error {
		capturedCallbacks = callbacks
		return nil
	}
	defer func() { startMonitorDashboardFunc = origStartFunc }()

	// 4. Run the command
	// We need to run monitorCmd.RunE directly or via executeCommand
	err := monitorCmd.RunE(monitorCmd, []string{})
	require.NoError(t, err)

	// 5. Verify Callbacks logic

	// Test GetLogs for K8s session
	k8sSession := model.UnifiedSession{
		ID:       "pod-123",
		Name:     "ticket-1",
		Location: "k8s",
	}
	logs, err := capturedCallbacks.GetLogs(k8sSession)
	require.NoError(t, err)
	assert.Equal(t, "k8s logs content", logs)

	// Test Stop for K8s session
	err = capturedCallbacks.Stop(k8sSession)
	require.NoError(t, err)

	// Test Pause for K8s session (should fail)
	err = capturedCallbacks.Pause(k8sSession)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")

	// Test Resume for K8s session (should fail)
	err = capturedCallbacks.Resume(k8sSession)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")

	// Test GetLogs for Local Session
	mockSM.Sessions["local-1"] = &runner.SessionState{
		Name:    "local-1",
		LogFile: "mock.log",
	}

	localSession := model.UnifiedSession{
		ID:       "local-1",
		Name:     "local-1",
		Location: "local",
	}
	logs, err = capturedCallbacks.GetLogs(localSession)
	require.NoError(t, err)
	assert.Contains(t, logs, "line 1")
}
