package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestK8sSpawner_Cleanup(t *testing.T) {
	s := &K8sSpawner{}
	err := s.Cleanup(context.Background(), WorkItem{})
	assert.NoError(t, err)
}

func TestNewK8sSpawner_Config(t *testing.T) {
	// 1. Test with invalid KUBECONFIG to verify error
	t.Run("Invalid KUBECONFIG", func(t *testing.T) {
		t.Setenv("KUBECONFIG", "/invalid/path/to/config")
		t.Setenv("KUBERNETES_SERVICE_HOST", "") // Ensure not in-cluster

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		spawner, err := NewK8sSpawner(logger, "img", "ns", "p", "m", corev1.PullAlways, nil)
		assert.Error(t, err)
		assert.Nil(t, spawner)
	})

	// 2. Test with valid (dummy) KUBECONFIG
	t.Run("Valid KUBECONFIG", func(t *testing.T) {
		tmpDir := t.TempDir()
		kubeConfigPath := filepath.Join(tmpDir, "kubeconfig")
		// Minimal valid kubeconfig
		kubeConfigContent := `
apiVersion: v1
clusters:
- cluster:
    server: https://1.2.3.4
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
		err := os.WriteFile(kubeConfigPath, []byte(kubeConfigContent), 0644)
		assert.NoError(t, err)

		t.Setenv("KUBECONFIG", kubeConfigPath)
		t.Setenv("KUBERNETES_SERVICE_HOST", "")

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		spawner, err := NewK8sSpawner(logger, "img", "", "p", "m", corev1.PullAlways, nil)

		assert.NoError(t, err)
		assert.NotNil(t, spawner)
		assert.Equal(t, "default", spawner.Namespace)
	})
}

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

	// Execute in goroutine
	done := make(chan error)
	go func() {
		done <- spawner.Spawn(context.Background(), item)
	}()

	// Verify Job Created
	jobName := "recac-agent-ticket-1"
	var job *batchv1.Job
	var err error

	// Wait for job creation
	for i := 0; i < 10; i++ {
		job, err = fakeClient.BatchV1().Jobs("default").Get(context.Background(), jobName, metav1.GetOptions{})
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.NoError(t, err, "Job should be created")

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

	// Complete the job so Spawn returns
	job.Status.Succeeded = 1
	_, _ = fakeClient.BatchV1().Jobs("default").Update(context.Background(), job, metav1.UpdateOptions{})

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Log("Spawn timed out waiting for completion")
	}
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
		done := make(chan error)
		go func() {
			done <- spawner.Spawn(context.Background(), item)
		}()

		// Wait for job creation
		var job *batchv1.Job
		var err error
		for i := 0; i < 10; i++ {
			job, err = clientset.BatchV1().Jobs("test-ns").Get(context.Background(), "recac-agent-task-123", metav1.GetOptions{})
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
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

		// Finish job
		job.Status.Succeeded = 1
		_, _ = clientset.BatchV1().Jobs("test-ns").Update(context.Background(), job, metav1.UpdateOptions{})

		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Log("Spawn timed out")
		}
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

func TestK8sSpawner_Cancel(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := &K8sSpawner{
		Client:    clientset,
		Namespace: "test-ns",
		Logger:    logger,
	}

	ctx := context.Background()
	jobID := "JOB-1"
	jobName := "recac-agent-job-1"

	// Create a job first
	_, err := clientset.BatchV1().Jobs("test-ns").Create(ctx, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: map[string]string{"work-item": jobID},
		},
	}, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Test Cancel
	err = spawner.Cancel(ctx, jobID)
	assert.NoError(t, err)

	// Verify it's gone
	// Note: fake clientset deletes immediately
	_, err = clientset.BatchV1().Jobs("test-ns").Get(ctx, jobName, metav1.GetOptions{})
	assert.Error(t, err)
	// Usually returns "jobs.batch "recac-agent-job-1" not found"
	// But let's just check it's an error.
}

type K8sMockPoller struct {
	UpdateStatusFunc func(ctx context.Context, item WorkItem, status string, comment string) error
}

func (m *K8sMockPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	return nil, nil
}
func (m *K8sMockPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	if m.UpdateStatusFunc != nil {
		return m.UpdateStatusFunc(ctx, item, status, comment)
	}
	return nil
}
func (m *K8sMockPoller) Ping(ctx context.Context) error { return nil }

func TestK8sSpawner_Spawn_WaitsForCompletion(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockPoller := &K8sMockPoller{}

	spawner := &K8sSpawner{
		Client:        clientset,
		Namespace:     "test-ns",
		Image:         "recac-agent:latest",
		AgentProvider: "gemini",
		AgentModel:    "gemini-pro",
		PullPolicy:    corev1.PullAlways,
		Logger:        logger,
		Poller:        mockPoller,
	}

	item := WorkItem{
		ID:      "TASK-WAIT",
		RepoURL: "https://github.com/example/repo",
	}

	jobName := "recac-agent-task-wait"

	// 1. Success Case
	t.Run("Waits for Success", func(t *testing.T) {
		done := make(chan error)
		go func() {
			done <- spawner.Spawn(context.Background(), item)
		}()

		// Wait a bit to ensure Spawn is running and loop started
		time.Sleep(100 * time.Millisecond)

		// Verify Job created
		job, err := clientset.BatchV1().Jobs("test-ns").Get(context.Background(), jobName, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Equal(t, 0, int(job.Status.Succeeded))

		// Update to Succeeded
		job.Status.Succeeded = 1
		_, err = clientset.BatchV1().Jobs("test-ns").Update(context.Background(), job, metav1.UpdateOptions{})
		assert.NoError(t, err)

		// Wait for Spawn to return
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second): // Adjusted ticker to 2s in implementation
			t.Fatal("Spawn did not return in time")
		}
	})

	// 2. Failure Case
	t.Run("Waits for Failure and Updates Poller", func(t *testing.T) {
		item.ID = "TASK-FAIL"
		jobName = "recac-agent-task-fail"

		pollerCalled := false
		mockPoller.UpdateStatusFunc = func(ctx context.Context, item WorkItem, status string, comment string) error {
			pollerCalled = true
			assert.Equal(t, "Failed", status)
			assert.Contains(t, comment, "Agent job failed")
			return nil
		}

		done := make(chan error)
		go func() {
			done <- spawner.Spawn(context.Background(), item)
		}()

		time.Sleep(100 * time.Millisecond)

		// Update to Failed
		job, err := clientset.BatchV1().Jobs("test-ns").Get(context.Background(), jobName, metav1.GetOptions{})
		assert.NoError(t, err)
		job.Status.Failed = 1
		_, err = clientset.BatchV1().Jobs("test-ns").Update(context.Background(), job, metav1.UpdateOptions{})
		assert.NoError(t, err)

		// Wait for Spawn to return
		select {
		case err := <-done:
			assert.NoError(t, err) // Should return nil even on failure (as per Docker parity)
		case <-time.After(5 * time.Second):
			t.Fatal("Spawn did not return in time")
		}

		assert.True(t, pollerCalled, "Poller should have been updated")
	})
}