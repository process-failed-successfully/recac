package ui

import (
	"context"
	"errors"
	"recac/internal/k8s"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
)

// statusMockSessionManager implements runner.ISessionManager for testing
type statusMockSessionManager struct {
	runner.ISessionManager
	listSessionsFunc func() ([]*runner.SessionState, error)
}

func (m *statusMockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	if m.listSessionsFunc != nil {
		return m.listSessionsFunc()
	}
	return nil, nil
}

// mockDockerClient implements StatusDockerClient for testing
type mockDockerClient struct {
	serverVersionFunc func(ctx context.Context) (types.Version, error)
}

func (m *mockDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	if m.serverVersionFunc != nil {
		return m.serverVersionFunc(ctx)
	}
	return types.Version{}, nil
}

func TestGetStatus(t *testing.T) {
	// Backup original factories
	origSessionManagerFunc := NewSessionManagerFunc
	origDockerClientFunc := NewDockerClientFunc
	origK8sClientFunc := K8sNewClient

	defer func() {
		NewSessionManagerFunc = origSessionManagerFunc
		NewDockerClientFunc = origDockerClientFunc
		SetK8sClient(origK8sClientFunc)
	}()

	t.Run("Happy Path", func(t *testing.T) {
		// Mock Session Manager
		mockSM := &statusMockSessionManager{
			listSessionsFunc: func() ([]*runner.SessionState, error) {
				return []*runner.SessionState{
					{
						Name:      "test-session",
						PID:       1234,
						Status:    "running",
						StartTime: time.Now().Add(-1 * time.Hour),
						LogFile:   "/tmp/test.log",
					},
				}, nil
			},
		}
		NewSessionManagerFunc = func() (runner.ISessionManager, error) {
			return mockSM, nil
		}

		// Mock Docker Client
		mockDocker := &mockDockerClient{
			serverVersionFunc: func(ctx context.Context) (types.Version, error) {
				return types.Version{
					Version:    "20.10.0",
					APIVersion: "1.41",
					Os:         "linux",
					Arch:       "amd64",
				}, nil
			},
		}
		NewDockerClientFunc = func(project string) (StatusDockerClient, error) {
			return mockDocker, nil
		}

		// Mock K8s Client
		clientset := fake.NewSimpleClientset()
		// Mock server version
		clientset.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{
			GitVersion: "v1.25.0",
		}

		SetK8sClient(func() (*k8s.Client, error) {
			return &k8s.Client{
				Clientset: clientset,
				Namespace: "default",
			}, nil
		})

		output := GetStatus()

		assert.Contains(t, output, "RECAC Status")
		assert.Contains(t, output, "test-session")
		assert.Contains(t, output, "Docker Version: 20.10.0")
		assert.Contains(t, output, "Server Version: v1.25.0")
	})

	t.Run("All Failures", func(t *testing.T) {
		// Mock Session Manager Failure
		NewSessionManagerFunc = func() (runner.ISessionManager, error) {
			return nil, errors.New("sm init failed")
		}

		// Mock Docker Client Failure
		NewDockerClientFunc = func(project string) (StatusDockerClient, error) {
			return nil, errors.New("docker init failed")
		}

		// Mock K8s Client Failure
		SetK8sClient(func() (*k8s.Client, error) {
			return nil, errors.New("k8s init failed")
		})

		output := GetStatus()

		assert.Contains(t, output, "failed to initialize session manager")
		assert.Contains(t, output, "Docker client failed to initialize")
		assert.Contains(t, output, "Could not connect to Kubernetes")
	})

	t.Run("Partial Failures", func(t *testing.T) {
		// SM: ListSessions fail
		mockSM := &statusMockSessionManager{
			listSessionsFunc: func() ([]*runner.SessionState, error) {
				return nil, errors.New("list failed")
			},
		}
		NewSessionManagerFunc = func() (runner.ISessionManager, error) {
			return mockSM, nil
		}

		// Docker: ServerVersion fail
		mockDocker := &mockDockerClient{
			serverVersionFunc: func(ctx context.Context) (types.Version, error) {
				return types.Version{}, errors.New("version failed")
			},
		}
		NewDockerClientFunc = func(project string) (StatusDockerClient, error) {
			return mockDocker, nil
		}

		// K8s: ServerVersion fail
		clientset := fake.NewSimpleClientset()
		clientset.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = nil // triggers error if not set? No, returns empty.
		// Actually fake discovery doesn't return error easily unless we inject reaction.

		// Let's just mock K8s GetServerVersion via wrapper if possible, but we use real wrapper methods.
		// So we accept that K8s might succeed partially.
		// Or we can just reuse K8s init fail for simplicity as we tested K8s failure in previous test.
		// But let's verify checking ListPods fail.

		SetK8sClient(func() (*k8s.Client, error) {
			return &k8s.Client{
				Clientset: clientset,
				Namespace: "default",
			}, nil
		})

		output := GetStatus()

		assert.Contains(t, output, "failed to list sessions")
		assert.Contains(t, output, "Could not connect to Docker daemon")
	})

	t.Run("No Sessions", func(t *testing.T) {
		mockSM := &statusMockSessionManager{
			listSessionsFunc: func() ([]*runner.SessionState, error) {
				return []*runner.SessionState{}, nil
			},
		}
		NewSessionManagerFunc = func() (runner.ISessionManager, error) {
			return mockSM, nil
		}

		// Use working docker/k8s to avoid noise
		mockDocker := &mockDockerClient{
			serverVersionFunc: func(ctx context.Context) (types.Version, error) {
				return types.Version{}, nil
			},
		}
		NewDockerClientFunc = func(project string) (StatusDockerClient, error) { return mockDocker, nil }
		SetK8sClient(func() (*k8s.Client, error) { return &k8s.Client{Clientset: fake.NewSimpleClientset()}, nil })

		output := GetStatus()
		assert.Contains(t, output, "No active or past sessions found")
	})
}
