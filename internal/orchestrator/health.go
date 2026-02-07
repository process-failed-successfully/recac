package orchestrator

import (
	"context"
	"fmt"
	"net/http"

	"recac/internal/jira"

	"github.com/docker/docker/client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// variables exposed for testing
var (
	httpClient    = &http.Client{}
	githubBaseURL = "https://api.github.com"
)

// CheckJira validates Jira connectivity and authentication.
func CheckJira(ctx context.Context, url, username, token string) error {
	if url == "" || username == "" || token == "" {
		return fmt.Errorf("missing jira credentials or url")
	}
	client := jira.NewClient(url, username, token)
	return client.Authenticate(ctx)
}

// CheckGitHub validates GitHub connectivity and authentication.
func CheckGitHub(ctx context.Context, token, owner, repo string) error {
	if token == "" || owner == "" || repo == "" {
		return fmt.Errorf("missing github credentials or repo info")
	}
	url := fmt.Sprintf("%s/repos/%s/%s", githubBaseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github api returned status: %s", resp.Status)
	}
	return nil
}

// CheckDocker validates Docker Daemon connectivity.
func CheckDocker(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	_, err = cli.Ping(ctx)
	return err
}

// CheckK8s validates Kubernetes Cluster connectivity.
func CheckK8s(ctx context.Context, namespace string) error {
	// Try loading kubeconfig from default locations or in-cluster config
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get k8s server version: %w", err)
	}

	return nil
}

// CheckAIProvider validates that the API key for the provider is set.
func CheckAIProvider(provider, apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("api key for provider %s is missing", provider)
	}
	return nil
}
