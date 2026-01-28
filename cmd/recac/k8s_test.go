package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sversion "k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"

	"recac/internal/k8s"
)

func TestK8sInfo(t *testing.T) {
	// Mock factory
	origFactory := k8sClientFactory
	defer func() { k8sClientFactory = origFactory }()

	k8sClientFactory = func() (IK8sClient, error) {
		fakeClientset := fake.NewSimpleClientset()
		fakeDiscovery, _ := fakeClientset.Discovery().(*fakediscovery.FakeDiscovery)
		fakeDiscovery.FakedServerVersion = &k8sversion.Info{Major: "1", Minor: "25"}

		return &k8s.Client{
			Clientset: fakeClientset,
			Namespace: "default",
		}, nil
	}

	cmd := k8sInfoCmd
	var out bytes.Buffer
	cmd.SetOut(&out)
    cmd.SetErr(&out)

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Kubernetes Version: v1.25")
	assert.Contains(t, out.String(), "Namespace: default")
}

func TestK8sAgents(t *testing.T) {
	origFactory := k8sClientFactory
	defer func() { k8sClientFactory = origFactory }()

	k8sClientFactory = func() (IK8sClient, error) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "agent-1",
				Namespace: "default",
				Labels:    map[string]string{"app": "recac-agent"},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: "10.0.0.1",
			},
		}
		return &k8s.Client{
			Clientset: fake.NewSimpleClientset(&pod),
			Namespace: "default",
		}, nil
	}

	cmd := k8sAgentsCmd
	var out bytes.Buffer
	cmd.SetOut(&out)
    cmd.SetErr(&out)
	cmd.SetContext(context.Background())

	// Set global flag variable manually since we are in package main test
	k8sLabelSelector = "app=recac-agent"

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "agent-1")
	assert.Contains(t, out.String(), "Running")
}
