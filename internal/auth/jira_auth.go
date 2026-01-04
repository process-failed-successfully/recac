package auth

import (
	"context"
	"errors"
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JiraCredentials holds the authentication details for Jira API
type JiraCredentials struct {
	Username string
	APIKey   string
	BaseURL  string
}

// JiraAuthenticator handles secure retrieval and management of Jira credentials
type JiraAuthenticator struct {
	clientset *kubernetes.Clientset
	namespace string
	secretName string
}

// NewJiraAuthenticator creates a new JiraAuthenticator instance
func NewJiraAuthenticator(namespace, secretName string) (*JiraAuthenticator, error) {
	// Create in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &JiraAuthenticator{
		clientset: clientset,
		namespace: namespace,
		secretName: secretName,
	}, nil
}

// GetCredentials retrieves Jira credentials from Kubernetes secrets
func (ja *JiraAuthenticator) GetCredentials() (*JiraCredentials, error) {
	// Get the secret from Kubernetes
	secret, err := ja.clientset.CoreV1().Secrets(ja.namespace).Get(
		context.Background(),
		ja.secretName,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	// Extract credentials from secret
	username, ok := secret.Data["username"]
	if !ok {
		return nil, errors.New("username not found in secret")
	}

	apiKey, ok := secret.Data["api-key"]
	if !ok {
		return nil, errors.New("api-key not found in secret")
	}

	baseURL, ok := secret.Data["base-url"]
	if !ok {
		return nil, errors.New("base-url not found in secret")
	}

	return &JiraCredentials{
		Username: string(username),
		APIKey:   string(apiKey),
		BaseURL:  string(baseURL),
	}, nil
}

// ValidateCredentials verifies that credentials are valid by making a test API call
func (ja *JiraAuthenticator) ValidateCredentials() error {
	creds, err := ja.GetCredentials()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	// In a real implementation, this would make a test API call to Jira
	// For now, we'll just validate that we have non-empty credentials
	if creds.Username == "" || creds.APIKey == "" || creds.BaseURL == "" {
		return errors.New("invalid credentials: one or more fields are empty")
	}

	return nil
}

// GetCredentialsFromEnv retrieves credentials from environment variables (fallback)
func GetCredentialsFromEnv() (*JiraCredentials, error) {
	username := os.Getenv("JIRA_USERNAME")
	apiKey := os.Getenv("JIRA_API_KEY")
	baseURL := os.Getenv("JIRA_BASE_URL")

	if username == "" || apiKey == "" || baseURL == "" {
		return nil, errors.New("missing Jira credentials in environment variables")
	}

	return &JiraCredentials{
		Username: username,
		APIKey:   apiKey,
		BaseURL:  baseURL,
	}, nil
}
