
package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Client is a wrapper around the Kubernetes clientset.
type Client struct {
	Clientset kubernetes.Interface
	Config    clientcmd.ClientConfig
}

// NewClient creates a new Kubernetes client. It will not return an error
// if a kubeconfig is not found, but subsequent calls will fail.
func NewClient() (*Client, error) {
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientConfig := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{})

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		// This can happen if the context is invalid or the cluster is unreachable.
		// We don't want to error out here, as the user may just not have k8s configured.
		return &Client{Config: clientConfig}, nil
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{Clientset: clientset, Config: clientConfig}, nil
}

// GetCurrentContext returns the current kubeconfig context.
func (c *Client) GetCurrentContext() (string, error) {
	if c.Config == nil {
		return "", fmt.Errorf("kubeconfig not loaded")
	}
	rawConfig, err := c.Config.RawConfig()
	if err != nil {
		return "", fmt.Errorf("failed to get raw kubeconfig: %w", err)
	}
	if rawConfig.CurrentContext == "" {
		// Check if a kubeconfig file exists at all.
		// If not, we can provide a more helpful message.
		if home, err := os.UserHomeDir(); err == nil {
			if _, err := os.Stat(filepath.Join(home, ".kube", "config")); os.IsNotExist(err) {
				return "", nil // No kubeconfig, not an error state.
			}
		}
		return "", fmt.Errorf("no current context set in kubeconfig")
	}
	return rawConfig.CurrentContext, nil
}

// GetOrchestratorDeployment returns the main orchestrator deployment.
func (c *Client) GetOrchestratorDeployment(ctx context.Context) (*appsv1.Deployment, error) {
	if c.Clientset == nil {
		return nil, nil // No clientset means no k8s, not an error.
	}

	namespace, _, err := c.Config.Namespace()
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace: %w", err)
	}

	deployment, err := c.Clientset.AppsV1().Deployments(namespace).Get(ctx, "recac-orchestrator", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return deployment, nil
}

// ListAgentPods returns a list of all recac-agent pods.
func (c *Client) ListAgentPods(ctx context.Context) ([]apiv1.Pod, error) {
	if c.Clientset == nil {
		return nil, nil // No clientset means no k8s, not an error.
	}
	namespace, _, err := c.Config.Namespace()
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace: %w", err)
	}

	podList, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=recac-agent",
	})
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}
