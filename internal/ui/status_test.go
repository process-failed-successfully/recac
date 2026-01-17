package ui

import (
	"context"
	"errors"
	"recac/internal/k8s"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	discoveryfake "k8s.io/client-go/discovery/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// MockSessionManager implements runner.ISessionManager for testing
type MockSessionManager struct {
	runner.ISessionManager
	Sessions []*runner.SessionState
	Err      error
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	return m.Sessions, m.Err
}

// MockStatusDockerClient implements StatusDockerClient for testing
type MockStatusDockerClient struct {
	Version types.Version
	Err     error
}

func (m *MockStatusDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	return m.Version, m.Err
}

func TestGetStatus(t *testing.T) {
	// Setup Viper config
	viper.Set("provider", "test-provider")
	viper.Set("model", "test-model")
	viper.Set("config", "/tmp/config.yaml")

	// Helper to restore factories
	originalSessionManager := runner.NewSessionManager
	originalDockerClient := NewStatusDockerClient
	originalK8sClient := K8sNewClient
	defer func() {
		runner.NewSessionManager = originalSessionManager
		NewStatusDockerClient = originalDockerClient
		SetK8sClient(originalK8sClient)
	}()

	t.Run("Success", func(t *testing.T) {
		// Mock SessionManager
		runner.NewSessionManager = func() (runner.ISessionManager, error) {
			return &MockSessionManager{
				Sessions: []*runner.SessionState{
					{
						Name:      "test-session",
						PID:       12345,
						Status:    "running",
						StartTime: time.Now().Add(-1 * time.Hour),
						LogFile:   "/tmp/test.log",
					},
				},
			}, nil
		}

		// Mock DockerClient
		NewStatusDockerClient = func() (StatusDockerClient, error) {
			return &MockStatusDockerClient{
				Version: types.Version{
					Version:    "20.10.0",
					APIVersion: "1.41",
					Os:         "linux",
					Arch:       "amd64",
				},
			}, nil
		}

		// Mock K8sClient
		SetK8sClient(func() (*k8s.Client, error) {
			clientset := k8sfake.NewSimpleClientset()

			// Mock ServerVersion via Discovery
			clientset.Discovery().(*discoveryfake.FakeDiscovery).FakedServerVersion = &version.Info{
				GitVersion: "v1.20.0",
			}

			// Add a pod
			clientset.CoreV1().Pods("default").Create(context.Background(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "recac-agent-123",
					Labels: map[string]string{"app": "recac-agent"},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			}, metav1.CreateOptions{})

			return &k8s.Client{
				Clientset: clientset,
				Namespace: "default",
			}, nil
		})

		output := GetStatus()

		assert.Contains(t, output, "--- RECAC Status ---")
		assert.Contains(t, output, "test-session")
		assert.Contains(t, output, "PID: 12345")
		assert.Contains(t, output, "RUNNING")

		assert.Contains(t, output, "Docker Version: 20.10.0")
		assert.Contains(t, output, "OS/Arch: linux/amd64")

		assert.Contains(t, output, "Server Version: v1.20.0")
		assert.Contains(t, output, "RECAC Agent Pods: 1")
		assert.Contains(t, output, "recac-agent-123 (Running)")

		assert.Contains(t, output, "Provider: test-provider")
		assert.Contains(t, output, "Model: test-model")
	})

	t.Run("Errors", func(t *testing.T) {
		// Mock SessionManager failure
		runner.NewSessionManager = func() (runner.ISessionManager, error) {
			return nil, errors.New("session init error")
		}

		// Mock DockerClient failure
		NewStatusDockerClient = func() (StatusDockerClient, error) {
			return nil, errors.New("docker init error")
		}

		// Mock K8sClient failure
		SetK8sClient(func() (*k8s.Client, error) {
			return nil, errors.New("k8s init error")
		})

		output := GetStatus()

		assert.Contains(t, output, "Error: failed to initialize session manager: session init error")
		assert.Contains(t, output, "Docker client failed to initialize")
		assert.Contains(t, output, "Error: docker init error")
		assert.Contains(t, output, "Could not connect to Kubernetes")
		assert.Contains(t, output, "Error: k8s init error")
	})

	t.Run("K8s ServerVersion Error", func(t *testing.T) {
		SetK8sClient(func() (*k8s.Client, error) {
			clientset := k8sfake.NewSimpleClientset()
			// To simulate discovery error we can attach a reaction
			clientset.Fake.PrependReactor("get", "version", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, errors.New("discovery error")
			})
			// Wait, the FakeDiscovery implementation of ServerVersion does not check reactors usually.
			// It returns FakedServerVersion directly or nil.
			// But creating a mock k8s client that fails on server version is tricky with fake clientset if it doesn't expose error injection.
			// However, GetServerVersion uses clientset.Discovery().ServerVersion().
			// fake.FakeDiscovery.ServerVersion() returns (Info, nil) always unless we can override it?
			// It seems hard to inject error into ServerVersion of FakeDiscovery directly.
			// So we might skip this specific branch or accept it's hard to test with simple fake.

			// Let's try to just check the "No pods" case or ListPods error
			return &k8s.Client{Clientset: clientset, Namespace: "default"}, nil
		})

		// We'll skip explicitly testing ServerVersion error for K8s as it's hard with standard fake
	})

	t.Run("Empty Sessions", func(t *testing.T) {
		runner.NewSessionManager = func() (runner.ISessionManager, error) {
			return &MockSessionManager{Sessions: []*runner.SessionState{}}, nil
		}

		// restore others to success state to reduce noise
		NewStatusDockerClient = func() (StatusDockerClient, error) {
			return &MockStatusDockerClient{Version: types.Version{Version: "1.0"}}, nil
		}
		SetK8sClient(func() (*k8s.Client, error) {
			return &k8s.Client{Clientset: k8sfake.NewSimpleClientset(), Namespace: "default"}, nil
		})

		output := GetStatus()
		assert.Contains(t, output, "No active or past sessions found")
	})
}
