package orchestrator

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewK8sSpawner(t *testing.T) {
	// We can't easily test InClusterConfig or local config loading without modifying global state or filesystem.
	// But we can test basic struct initialization logic if we bypass the client creation part or if we can inject it.
	// NewK8sSpawner is tightly coupled to clientcmd.BuildConfigFromFlags.
	// However, we can test the constructor returns error if no config found (in CI/Docker environment).

	// In the test environment, we expect this to fail because there's no kubeconfig.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	spawner, err := NewK8sSpawner(logger, "image", "ns", "provider", "model", corev1.PullAlways)

	// It should probably fail
	assert.Error(t, err)
	assert.Nil(t, spawner)
}

func TestK8sSpawner_Spawn(t *testing.T) {
	// Create fake clientset
	clientset := fake.NewSimpleClientset()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	spawner := &K8sSpawner{
		Client:        clientset,
		Namespace:     "default",
		Image:         "test-image",
		AgentProvider: "test-provider",
		AgentModel:    "test-model",
		PullPolicy:    corev1.PullAlways,
		Logger:        logger,
	}

	ctx := context.Background()
	item := WorkItem{
		ID:      "TEST-123",
		RepoURL: "https://github.com/org/repo",
		EnvVars: map[string]string{"FOO": "BAR"},
	}

	// Test Successful Spawn
	err := spawner.Spawn(ctx, item)
	assert.NoError(t, err)

	// Verify Job Created
	jobName := "recac-agent-test-123"
	job, err := clientset.BatchV1().Jobs("default").Get(ctx, jobName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, jobName, job.Name)
	assert.Equal(t, "test-image", job.Spec.Template.Spec.Containers[0].Image)

	// Verify Env Vars
	foundEnv := false
	for _, env := range job.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "FOO" && env.Value == "BAR" {
			foundEnv = true
		}
		if env.Name == "RECAC_PROJECT_ID" && env.Value == "TEST-123" {
		}
	}
	assert.True(t, foundEnv, "Env var FOO=BAR not found")

	// Test Idempotency (Job exists and active)
	err = spawner.Spawn(ctx, item)
	assert.NoError(t, err)

	// Test Failed Job Retry
	// Mark job as failed
	job.Status.Failed = 1
	_, err = clientset.BatchV1().Jobs("default").UpdateStatus(ctx, job, metav1.UpdateOptions{})
	// fake client update status might need Update, not UpdateStatus depending on version/impl,
	// but usually Update works for everything in fake client if it's simple.
	// Actually, fake client stores the object. UpdateStatus is correct for subresources.
	// But simple fake client might not distinguish. Let's try Update.
	_, err = clientset.BatchV1().Jobs("default").Update(ctx, job, metav1.UpdateOptions{})
	assert.NoError(t, err)

	// Spawn again - should delete and return error saying "will retry"
	err = spawner.Spawn(ctx, item)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cleaning up failed job")

	// Verify deletion
	_, err = clientset.BatchV1().Jobs("default").Get(ctx, jobName, metav1.GetOptions{})
	assert.Error(t, err) // Should be not found
}

func TestK8sSpawner_Cleanup(t *testing.T) {
	spawner := &K8sSpawner{}
	err := spawner.Cleanup(context.Background(), WorkItem{})
	assert.NoError(t, err)
}

func TestSanitizeK8sName(t *testing.T) {
	assert.Equal(t, "test-123", sanitizeK8sName("TEST-123"))
	assert.Equal(t, "test-foo", sanitizeK8sName("Test_Foo"))
	assert.Equal(t, "a-b-c", sanitizeK8sName("a.b.c"))
	assert.Equal(t, "foo", sanitizeK8sName("-foo-"))
}

func TestExtractRepoPath(t *testing.T) {
	assert.Equal(t, "org/repo", extractRepoPath("https://github.com/org/repo"))
}

func TestBoolPtr(t *testing.T) {
	b := true
	ptr := boolPtr(b)
	assert.NotNil(t, ptr)
	assert.True(t, *ptr)
}
