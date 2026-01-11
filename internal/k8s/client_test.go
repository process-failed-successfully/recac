
package k8s

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func newMockClient(config clientcmdapi.Config, objects ...runtime.Object) *Client {
	return &Client{
		Clientset: fake.NewSimpleClientset(objects...),
		Config: clientcmd.NewNonInteractiveClientConfig(
			config,
			config.CurrentContext,
			&clientcmd.ConfigOverrides{},
			nil,
		),
	}
}

func TestGetCurrentContext(t *testing.T) {
	testCases := []struct {
		name          string
		setup         func(t *testing.T) *Client
		expected      string
		expectErr     bool
		errContains   string
	}{
		{
			name: "Valid Context",
			setup: func(t *testing.T) *Client {
				config := clientcmdapi.Config{
					CurrentContext: "my-context",
				}
				return newMockClient(config)
			},
			expected:  "my-context",
			expectErr: false,
		},
		{
			name: "No Current Context Set",
			setup: func(t *testing.T) *Client {
				// We need a real file to exist for this check to trigger
				tempDir := t.TempDir()
				kubeconfigPath := filepath.Join(tempDir, ".kube", "config")
				require.NoError(t, os.MkdirAll(filepath.Dir(kubeconfigPath), 0755))
				_, err := os.Create(kubeconfigPath)
				require.NoError(t, err)
				t.Setenv("HOME", tempDir)
				t.Setenv("KUBECONFIG", kubeconfigPath)

				config := clientcmdapi.Config{} // No CurrentContext
				return newMockClient(config)
			},
			expectErr:   true,
			errContains: "no current context set",
		},
		{
			name: "No Kubeconfig File",
			setup: func(t *testing.T) *Client {
				// Set HOME to a temp dir with no .kube/config
				t.Setenv("HOME", t.TempDir())
				config := clientcmdapi.Config{}
				return newMockClient(config)
			},
			expected:  "", // Should return empty string, no error
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := tc.setup(t)
			context, err := client.GetCurrentContext()

			if tc.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, context)
			}
		})
	}
}

func TestGetOrchestratorDeployment(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "recac-orchestrator", Namespace: "default"},
	}
	client := newMockClient(clientcmdapi.Config{CurrentContext: "c", Contexts: map[string]*clientcmdapi.Context{"c": {Namespace: "default"}}}, deployment)

	result, err := client.GetOrchestratorDeployment(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "recac-orchestrator", result.Name)

	// Test Not Found
	emptyClient := newMockClient(clientcmdapi.Config{CurrentContext: "c", Contexts: map[string]*clientcmdapi.Context{"c": {Namespace: "default"}}})
	_, err = emptyClient.GetOrchestratorDeployment(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestListAgentPods(t *testing.T) {
	pod1 := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "agent-1", Namespace: "default", Labels: map[string]string{"app": "recac-agent"}},
	}
	pod2 := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "agent-2", Namespace: "default", Labels: map[string]string{"app": "recac-agent"}},
	}
	otherPod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "default", Labels: map[string]string{"app": "other-app"}},
	}
	client := newMockClient(clientcmdapi.Config{CurrentContext: "c", Contexts: map[string]*clientcmdapi.Context{"c": {Namespace: "default"}}}, pod1, pod2, otherPod)

	pods, err := client.ListAgentPods(context.Background())
	require.NoError(t, err)
	assert.Len(t, pods, 2)
	assert.ElementsMatch(t, []string{"agent-1", "agent-2"}, []string{pods[0].Name, pods[1].Name})
}
