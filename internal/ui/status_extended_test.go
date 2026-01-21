package ui

import (
	"os"
	"recac/internal/k8s"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetStatus(t *testing.T) {
	// Setup Session Manager with dummy sessions
	tmpDir := t.TempDir()

	// Mock HOME to point to tmpDir for SessionManager
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	sm, _ := runner.NewSessionManager()

	// Create a running session mock
	session := &runner.SessionState{
		Name:      "test-session",
		PID:       os.Getpid(), // Use real PID so it stays "running"
		Status:    "running",
		StartTime: time.Now().Add(-1 * time.Hour),
		LogFile:   "/tmp/test.log",
	}
	sm.SaveSession(session)

	// Mock Viper Config
	viper.Set("provider", "test-provider")
	viper.Set("model", "test-model")
	defer viper.Reset()

	// Mock K8s Client
	originalK8sNewClient := K8sNewClient
	defer func() { K8sNewClient = originalK8sNewClient }()

	SetK8sClient(func() (*k8s.Client, error) {
		client := fake.NewSimpleClientset(
			&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "recac-agent-pod-1",
					Namespace: "default",
					Labels:    map[string]string{"app": "recac-agent"},
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
				},
			},
		)

		// Mock Server Version
		client.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{
			Major: "1",
			Minor: "28",
			GitVersion: "v1.28.0",
		}

		return &k8s.Client{
			Clientset: client,
			Namespace: "default",
		}, nil
	})

	// Run GetStatus
	status := GetStatus()

	// Verify Output
	assert.Contains(t, status, "--- RECAC Status ---")
	assert.Contains(t, status, "test-session")
	// assert.Contains(t, status, fmt.Sprintf("PID: %d", os.Getpid())) // Not importing fmt yet
	assert.Contains(t, status, "RUNNING")

	// Docker check might fail as we didn't mock DockerClient in ui/status.go,
	// it calls docker.NewClient directly.
	// Depending on env, it prints error or info.
	// But GetStatus handles errors gracefully.

	// Check K8s output
	assert.Contains(t, status, "[Kubernetes Environment]")
	assert.Contains(t, status, "Server Version: v1.28.0")
	assert.Contains(t, status, "RECAC Agent Pods: 1")
	assert.Contains(t, status, "recac-agent-pod-1 (Running)")

	// Check Config
	assert.Contains(t, status, "Provider: test-provider")
	assert.Contains(t, status, "Model: test-model")
}

func TestGetStatus_K8sError(t *testing.T) {
	// Mock K8s Client Error
	originalK8sNewClient := K8sNewClient
	defer func() { K8sNewClient = originalK8sNewClient }()

	SetK8sClient(func() (*k8s.Client, error) {
		return nil, os.ErrNotExist
	})

	status := GetStatus()
	assert.Contains(t, status, "Could not connect to Kubernetes")
}
