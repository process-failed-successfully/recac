package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	testclient "k8s.io/client-go/testing"
)

func TestGetServerVersion(t *testing.T) {
	fakeClientset := fake.NewSimpleClientset()
	fakeDiscovery, ok := fakeClientset.Discovery().(*fakediscovery.FakeDiscovery)
	require.True(t, ok)

	fakeDiscovery.FakedServerVersion = &version.Info{
		Major: "1",
		Minor: "20",
	}

	client := &Client{Clientset: fakeClientset}

	ver, err := client.GetServerVersion()
	require.NoError(t, err)
	assert.Equal(t, "1", ver.Major)
	assert.Equal(t, "20", ver.Minor)
}

func TestListPods(t *testing.T) {
	pod1 := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "default", Labels: map[string]string{"app": "recac-agent"}}}
	pod2 := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod2", Namespace: "default", Labels: map[string]string{"app": "other"}}}
	fakeClientset := fake.NewSimpleClientset(&pod1, &pod2)

	client := &Client{Clientset: fakeClientset, Namespace: "default"}

	pods, err := client.ListPods(context.Background(), "app=recac-agent")
	require.NoError(t, err)
	assert.Len(t, pods, 1)
	assert.Equal(t, "pod1", pods[0].Name)
}

func TestListPodsError(t *testing.T) {
	fakeClientset := fake.NewSimpleClientset()
	fakeClientset.PrependReactor("list", "pods", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, assert.AnError
	})

	client := &Client{Clientset: fakeClientset, Namespace: "default"}

	_, err := client.ListPods(context.Background(), "app=recac-agent")
	require.Error(t, err)
}
