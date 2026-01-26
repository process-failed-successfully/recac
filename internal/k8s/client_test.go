package k8s

import (
	"context"
	"os"
	"path/filepath"
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

func TestGetServerVersionError(t *testing.T) {
	fakeClientset := fake.NewSimpleClientset()
	fakeClientset.PrependReactor("get", "version", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, assert.AnError
	})

	client := &Client{Clientset: fakeClientset}

	_, err := client.GetServerVersion()
	require.Error(t, err)
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

func TestDeletePod(t *testing.T) {
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "default"}}
	fakeClientset := fake.NewSimpleClientset(&pod)

	client := &Client{Clientset: fakeClientset, Namespace: "default"}

	err := client.DeletePod(context.Background(), "pod1")
	require.NoError(t, err)

	// Verify pod is gone
	_, err = fakeClientset.CoreV1().Pods("default").Get(context.Background(), "pod1", metav1.GetOptions{})
	require.Error(t, err)
}

func TestDeletePodError(t *testing.T) {
	fakeClientset := fake.NewSimpleClientset()
	fakeClientset.PrependReactor("delete", "pods", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, assert.AnError
	})

	client := &Client{Clientset: fakeClientset, Namespace: "default"}

	err := client.DeletePod(context.Background(), "pod1")
	require.Error(t, err)
}

func TestNewClient(t *testing.T) {
	// No kubeconfig file
	t.Run("No Kubeconfig", func(t *testing.T) {
		// Create a temporary home directory
		tmpDir, err := os.MkdirTemp("", "kubeconfig")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Set the HOME environment variable to the temporary directory
		t.Setenv("HOME", tmpDir)

		_, err = NewClient()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "kubeconfig not found")
	})

	// Invalid kubeconfig file
	t.Run("Invalid Kubeconfig", func(t *testing.T) {
		// Create a temporary home directory
		tmpDir, err := os.MkdirTemp("", "kubeconfig")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create an invalid kubeconfig file
		kubeconfigFile := filepath.Join(tmpDir, ".kube", "config")
		err = os.MkdirAll(filepath.Dir(kubeconfigFile), 0755)
		require.NoError(t, err)
		err = os.WriteFile(kubeconfigFile, []byte("invalid kubeconfig"), 0644)
		require.NoError(t, err)

		// Set the HOME environment variable to the temporary directory
		t.Setenv("HOME", tmpDir)

		_, err = NewClient()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load kubeconfig")
	})

	// Valid kubeconfig file
	t.Run("Valid Kubeconfig", func(t *testing.T) {
		// Create a temporary home directory
		tmpDir, err := os.MkdirTemp("", "kubeconfig")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		// Create a valid kubeconfig file
		kubeconfigFile := filepath.Join(tmpDir, ".kube", "config")
		err = os.MkdirAll(filepath.Dir(kubeconfigFile), 0755)
		require.NoError(t, err)
		// This is a minimal valid kubeconfig file
		kubeconfigContent := `
apiVersion: v1
clusters:
- cluster:
    server: http://localhost:8080
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
kind: Config
preferences: {}
users:
- name: test-user
  user:
    token: test-token
`
		err = os.WriteFile(kubeconfigFile, []byte(kubeconfigContent), 0644)
		require.NoError(t, err)

		// Set the HOME environment variable to the temporary directory
		t.Setenv("HOME", tmpDir)

		client, err := NewClient()
		require.NoError(t, err)
		assert.NotNil(t, client)
	})
}
