package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/k8s"
	"recac/internal/runner"
	"recac/internal/ui"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sversion "k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetStatus(t *testing.T) {
	// Setup: Create a temporary directory for sessions
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir) // Isolate session manager

	// We need to initialize the session manager to create the .recac/sessions directory
	_, err := runner.NewSessionManager()
	require.NoError(t, err, "failed to create session manager")

	// Create a fake session
	sessionName := fmt.Sprintf("test-session-%d", time.Now().UnixNano())
	fakeSession := &runner.SessionState{
		Name:      sessionName,
		PID:       os.Getpid(),
		StartTime: time.Now().Add(-10 * time.Minute),
		Status:    "running",
		LogFile:   "/tmp/test.log",
	}
	// Correctly construct the path where the session manager will look for the file
	sessionPath := filepath.Join(tempDir, ".recac", "sessions", fmt.Sprintf("%s.json", sessionName))
	require.NoError(t, os.MkdirAll(filepath.Dir(sessionPath), 0755))

	data, err := json.Marshal(fakeSession)
	require.NoError(t, err, "failed to marshal fake session")

	require.NoError(t, os.WriteFile(sessionPath, data, 0644), "failed to write fake session file")

	// Setup viper config
	viper.Set("provider", "test-provider")
	viper.Set("model", "test-model")
	viper.Set("config", "/tmp/config.yaml")
	defer viper.Reset()

	// Mock the k8s client
	oldK8sNewClient := ui.K8sNewClient
	defer func() { ui.SetK8sClient(oldK8sNewClient) }()
	ui.SetK8sClient(func() (*k8s.Client, error) {
		fakeClientset := fake.NewSimpleClientset(
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "recac-agent-123",
					Namespace: "default",
					Labels:    map[string]string{"app": "recac-agent"},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
		)
		fakeDiscovery, ok := fakeClientset.Discovery().(*fakediscovery.FakeDiscovery)
		require.True(t, ok)
		fakeDiscovery.FakedServerVersion = &k8sversion.Info{
			GitVersion: "v1.2.3",
		}
		return &k8s.Client{
			Clientset: fakeClientset,
			Namespace: "default",
		}, nil
	})

	// Execute the function
	output := ui.GetStatus()

	// Assertions
	t.Run("Session Output", func(t *testing.T) {
		assert.Contains(t, output, "[Sessions]")
		assert.Contains(t, output, sessionName)
		assert.Contains(t, output, fmt.Sprintf("PID: %d", fakeSession.PID))
		assert.Contains(t, output, "Status: RUNNING") // The logic uppercases the status
	})

	t.Run("Docker Output", func(t *testing.T) {
		assert.Contains(t, output, "[Docker Environment]")
	})

	t.Run("Configuration Output", func(t *testing.T) {
		assert.Contains(t, output, "[Configuration]")
		assert.Contains(t, output, "Provider: test-provider")
		assert.Contains(t, output, "Model: test-model")
		assert.Contains(t, output, "Config File: /tmp/config.yaml")
	})

	t.Run("Kubernetes Output", func(t *testing.T) {
		assert.Contains(t, output, "[Kubernetes Environment]")
		assert.Contains(t, output, "Server Version: v1.2.3")
		assert.Contains(t, output, "RECAC Agent Pods: 1")
		assert.Contains(t, output, "recac-agent-123 (Running)")
	})
}

func TestGetStatus_NoSessions(t *testing.T) {
	// Setup: Create a temporary directory for sessions
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir) // Isolate session manager

	// Initialize session manager to ensure directories are created
	_, err := runner.NewSessionManager()
	require.NoError(t, err)

	// Execute the function with no sessions present
	output := ui.GetStatus()

	// Assertions
	assert.Contains(t, output, "No active or past sessions found.")
}
