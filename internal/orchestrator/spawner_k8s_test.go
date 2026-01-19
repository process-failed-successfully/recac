package orchestrator

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestK8sSpawner_Spawn_PropagatesSecrets(t *testing.T) {
	// Setup fake client
	clientset := fake.NewSimpleClientset()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	spawner := &K8sSpawner{
		Client:        clientset,
		Namespace:     "default",
		Image:         "recac-agent:latest",
		AgentProvider: "openai",
		AgentModel:    "gpt-4",
		PullPolicy:    corev1.PullIfNotPresent,
		Logger:        logger,
	}

	// Set env vars
	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	os.Setenv("GITHUB_API_KEY", "ghp-test-key")
	defer os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("GITHUB_API_KEY")

	item := WorkItem{
		ID:      "TEST-123",
		RepoURL: "https://github.com/test/repo",
		EnvVars: map[string]string{
			"CUSTOM_VAR": "custom_value",
		},
	}

	// Execute
	err := spawner.Spawn(context.Background(), item)
	require.NoError(t, err)

	// Verify Job created
	jobs, err := clientset.BatchV1().Jobs("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, jobs.Items, 1)

	job := jobs.Items[0]
	container := job.Spec.Template.Spec.Containers[0]

	// Check Env Vars
	envMap := make(map[string]string)
	for _, env := range container.Env {
		envMap[env.Name] = env.Value
	}

	assert.Equal(t, "sk-test-key", envMap["OPENAI_API_KEY"], "OPENAI_API_KEY should be propagated")
	assert.Equal(t, "ghp-test-key", envMap["GITHUB_API_KEY"], "GITHUB_API_KEY should be propagated")
	assert.Equal(t, "ghp-test-key", envMap["RECAC_GITHUB_API_KEY"], "RECAC_GITHUB_API_KEY should be propagated for parity")
	assert.Equal(t, "custom_value", envMap["CUSTOM_VAR"])
}
