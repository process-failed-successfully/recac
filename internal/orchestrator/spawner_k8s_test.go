package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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

func TestK8sSpawner_Spawn_Lifecycle(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	
	spawner := &K8sSpawner{
		Client:        clientset,
		Namespace:     "test-ns",
		Image:         "recac-agent:latest",
		AgentProvider: "gemini",
		AgentModel:    "gemini-pro",
		PullPolicy:    corev1.PullAlways,
		Logger:        logger,
	}

	item := WorkItem{
		ID:      "TASK-123",
		RepoURL: "https://github.com/example/repo",
		EnvVars: map[string]string{
			"CUSTOM_VAR": "value",
		},
	}

	t.Run("Create Success", func(t *testing.T) {
		err := spawner.Spawn(context.Background(), item)
		assert.NoError(t, err)

		// Verify Job exists
		job, err := clientset.BatchV1().Jobs("test-ns").Get(context.Background(), "recac-agent-task-123", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Equal(t, "recac-agent-task-123", job.Name)
		
		// Verify container image and env
		container := job.Spec.Template.Spec.Containers[0]
		assert.Equal(t, "recac-agent:latest", container.Image)
		
		envMap := make(map[string]string)
		for _, e := range container.Env {
			envMap[e.Name] = e.Value
		}
		assert.Equal(t, "value", envMap["CUSTOM_VAR"])
		assert.Equal(t, "gemini", envMap["RECAC_PROVIDER"])
		assert.Equal(t, "gemini-pro", envMap["RECAC_MODEL"])
	})

	t.Run("Retry Existing Failed Job", func(t *testing.T) {
		// Set existing job to failed
		job, _ := clientset.BatchV1().Jobs("test-ns").Get(context.Background(), "recac-agent-task-123", metav1.GetOptions{})
		job.Status.Failed = 1
		clientset.BatchV1().Jobs("test-ns").Update(context.Background(), job, metav1.UpdateOptions{})

		err := spawner.Spawn(context.Background(), item)
		// Should return error indicating cleanup and retry
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cleaning up failed job")

		// Verify Job was deleted (or deletion requested)
		_, err = clientset.BatchV1().Jobs("test-ns").Get(context.Background(), "recac-agent-task-123", metav1.GetOptions{})
		assert.Error(t, err) // Should be deleted in fake clientset immediately usually
	})
}

func TestK8sSpawner_Cleanup(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := &K8sSpawner{
		Client:    clientset,
		Namespace: "default",
		Logger:    logger,
	}

	item := WorkItem{ID: "TASK-123"}

	// Create dummy job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "recac-agent-task-123",
			Namespace: "default",
		},
	}
	_, err := clientset.BatchV1().Jobs("default").Create(context.Background(), job, metav1.CreateOptions{})
	assert.NoError(t, err)

	err = spawner.Cleanup(context.Background(), item)
	assert.NoError(t, err)

	// Verify job STILL exists (Cleanup is no-op, relies on TTL)
	_, err = clientset.BatchV1().Jobs("default").Get(context.Background(), "recac-agent-task-123", metav1.GetOptions{})
	assert.NoError(t, err)
}

func TestSanitizeK8sName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"PROJ-123", "proj-123"},
		{"Task_With_Underscores", "task-with-underscores"},
		{"Multiple---Dashes", "multiple-dashes"},
		{"Ends-With-Dash-", "ends-with-dash"},
		{"$pecial#Chars!", "pecial-chars"},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, sanitizeK8sName(tc.input))
	}
}
