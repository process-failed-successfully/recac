package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/api/core/v1"
)

func TestK8sSpawner_Spawn_PropagatesEnvVars(t *testing.T) {
	// Setup
	fakeClient := fake.NewSimpleClientset()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := &K8sSpawner{
		Client:        fakeClient,
		Namespace:     "default",
		Image:         "test-image",
		AgentProvider: "openai",
		AgentModel:    "gpt-4",
		PullPolicy:    corev1.PullIfNotPresent,
		Logger:        logger,
	}

	// Set Environment Variables
	os.Setenv("GITHUB_API_KEY", "test-github-key")
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	defer os.Unsetenv("GITHUB_API_KEY")
	defer os.Unsetenv("OPENAI_API_KEY")

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
		EnvVars: map[string]string{"CUSTOM_VAR": "value"},
	}

	// Execute
	err := spawner.Spawn(context.Background(), item)
	assert.NoError(t, err)

	// Verify
	jobName := "recac-agent-ticket-1"
	job, err := fakeClient.BatchV1().Jobs("default").Get(context.Background(), jobName, metav1.GetOptions{})
	assert.NoError(t, err)

	// Check Env Vars
	envVars := job.Spec.Template.Spec.Containers[0].Env
	envMap := make(map[string]string)
	for _, e := range envVars {
		envMap[e.Name] = e.Value
	}

	// Assertions for consistency with DockerSpawner
	assert.Equal(t, "test-github-key", envMap["GITHUB_API_KEY"], "GITHUB_API_KEY should be propagated")
	assert.Equal(t, "test-github-key", envMap["RECAC_GITHUB_API_KEY"], "RECAC_GITHUB_API_KEY should be aliased to GITHUB_API_KEY")
	assert.Equal(t, "test-openai-key", envMap["OPENAI_API_KEY"], "OPENAI_API_KEY should be propagated")
	assert.Equal(t, "0", envMap["GIT_TERMINAL_PROMPT"], "GIT_TERMINAL_PROMPT should be 0")
	assert.Equal(t, "20", envMap["RECAC_MAX_ITERATIONS"], "RECAC_MAX_ITERATIONS should be 20")
}
