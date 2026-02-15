package ui

import (
	"context"
	"errors"
	"recac/internal/k8s"
	"recac/internal/runner"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestGetStatus_ListPodsFailure(t *testing.T) {
	// Backup factories
	origSessionManagerFunc := NewSessionManagerFunc
	origDockerClientFunc := NewDockerClientFunc
	origK8sClientFunc := K8sNewClient
	defer func() {
		NewSessionManagerFunc = origSessionManagerFunc
		NewDockerClientFunc = origDockerClientFunc
		SetK8sClient(origK8sClientFunc)
	}()

	// Mock Session Manager (Success)
	NewSessionManagerFunc = func() (runner.ISessionManager, error) {
		return &statusMockSessionManager{
			listSessionsFunc: func() ([]*runner.SessionState, error) {
				return nil, nil
			},
		}, nil
	}

	// Mock Docker Client (Success)
	NewDockerClientFunc = func(project string) (StatusDockerClient, error) {
		return &mockDockerClient{
			serverVersionFunc: func(ctx context.Context) (types.Version, error) {
				return types.Version{}, nil
			},
		}, nil
	}

	// Mock K8s Client with ListPods failure
	clientset := fake.NewSimpleClientset()
	// Add reactor to fail List pods
	clientset.PrependReactor("list", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("list pods failed")
	})

	SetK8sClient(func() (*k8s.Client, error) {
		return &k8s.Client{
			Clientset: clientset,
			Namespace: "default",
		}, nil
	})

	output := GetStatus()

	assert.Contains(t, output, "Could not list orchestrator pods")
	assert.Contains(t, output, "list pods failed")
}
