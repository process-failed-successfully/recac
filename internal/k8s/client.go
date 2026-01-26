package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client is a wrapper around the Kubernetes clientset.
type Client struct {
	Clientset kubernetes.Interface
	Namespace string
}

// NewClient creates a new Kubernetes client. It first attempts to use an
// in-cluster configuration, and falls back to a kubeconfig file if that fails.
func NewClient() (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		var kubeconfig string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		} else {
			kubeconfig = os.Getenv("KUBECONFIG")
		}

		if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
			return nil, fmt.Errorf("kubeconfig not found at %s", kubeconfig)
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	namespace := "default"
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		namespace = strings.TrimSpace(string(data))
	}

	return &Client{
		Clientset: clientset,
		Namespace: namespace,
	}, nil
}

// GetServerVersion returns the version of the Kubernetes server.
func (c *Client) GetServerVersion() (*version.Info, error) {
	return c.Clientset.Discovery().ServerVersion()
}

// ListPods lists all pods in the client's namespace that match the given label selector.
func (c *Client) ListPods(ctx context.Context, labelSelector string) ([]corev1.Pod, error) {
	pods, err := c.Clientset.CoreV1().Pods(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list Kubernetes pods: %w", err)
	}
	return pods.Items, nil
}

// DeletePod deletes a pod by name in the client's namespace.
func (c *Client) DeletePod(ctx context.Context, name string) error {
	err := c.Clientset.CoreV1().Pods(c.Namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete Kubernetes pod %s: %w", name, err)
	}
	return nil
}
