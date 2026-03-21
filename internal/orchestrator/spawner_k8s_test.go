package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// Reusing MockSessionManager here if not exported, or defining one
type MockSessionManagerK8s struct {
	mock.Mock
}

func (m *MockSessionManagerK8s) SaveSession(session *runner.SessionState) error {
	args := m.Called(session)
	return args.Error(0)
}

func (m *MockSessionManagerK8s) LoadSession(name string) (*runner.SessionState, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*runner.SessionState), args.Error(1)
}

// Stubs for other methods
func (m *MockSessionManagerK8s) ListSessions() ([]*runner.SessionState, error) { return nil, nil }
func (m *MockSessionManagerK8s) StopSession(name string) error                 { return nil }
func (m *MockSessionManagerK8s) PauseSession(name string) error                { return nil }
func (m *MockSessionManagerK8s) ResumeSession(name string) error               { return nil }
func (m *MockSessionManagerK8s) GetSessionLogs(name string) (string, error)    { return "", nil }
func (m *MockSessionManagerK8s) GetSessionLogContent(name string, lines int) (string, error) {
	return "", nil
}
func (m *MockSessionManagerK8s) StartSession(name, goal string, command []string, workspace string) (*runner.SessionState, error) {
	return nil, nil
}
func (m *MockSessionManagerK8s) GetSessionPath(name string) string                 { return "" }
func (m *MockSessionManagerK8s) IsProcessRunning(pid int) bool                     { return false }
func (m *MockSessionManagerK8s) RemoveSession(name string, force bool) error       { return nil }
func (m *MockSessionManagerK8s) RenameSession(oldName, newName string) error       { return nil }
func (m *MockSessionManagerK8s) SessionsDir() string                               { return "" }
func (m *MockSessionManagerK8s) GetSessionGitDiffStat(name string) (string, error) { return "", nil }
func (m *MockSessionManagerK8s) ArchiveSession(name string) error                  { return nil }
func (m *MockSessionManagerK8s) UnarchiveSession(name string) error                { return nil }
func (m *MockSessionManagerK8s) ListArchivedSessions() ([]*runner.SessionState, error) {
	return nil, nil
}

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
		sm := new(MockSessionManagerK8s)
		spawner, err := NewK8sSpawner(logger, "img", "ns", nil, "p", "m", corev1.PullAlways, sm, 30, 5, 10)
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
		sm := new(MockSessionManagerK8s)
		spawner, err := NewK8sSpawner(logger, "img", "", nil, "p", "m", corev1.PullAlways, sm, 30, 5, 10)

		assert.NoError(t, err)
		assert.NotNil(t, spawner)
		assert.Equal(t, "default", spawner.Namespace)
	})
}

func TestK8sSpawner_Spawn_PropagatesEnvVars(t *testing.T) {
	// Setup
	fakeClient := fake.NewSimpleClientset()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sm := new(MockSessionManagerK8s)
	spawner := &K8sSpawner{
		Client:            fakeClient,
		Namespace:         "default",
		Image:             "test-image",
		AgentProvider:     "openai",
		AgentModel:        "gpt-4",
		PullPolicy:        corev1.PullIfNotPresent,
		Logger:            logger,
		SessionManager:    sm,
		MaxIterations:     42,
		ManagerFrequency:  7,
		TaskMaxIterations: 12,
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

	// Mock Session Manager
	sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "running"
	})).Return(nil)

	// Since we are using fake client, waitForJob will block unless we update status.
	// We run Spawn in a goroutine and update status in main thread.

	// Create Job first to ensure it exists for update?
	// No, Spawn creates it. We need to wait for Spawn to create it.

	go func() {
		// Wait for job creation
		for {
			_, err := fakeClient.BatchV1().Jobs("default").Get(context.Background(), "recac-agent-ticket-1", metav1.GetOptions{})
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Update status to Succeeded
		job, _ := fakeClient.BatchV1().Jobs("default").Get(context.Background(), "recac-agent-ticket-1", metav1.GetOptions{})
		job.Status.Succeeded = 1
		fakeClient.BatchV1().Jobs("default").Update(context.Background(), job, metav1.UpdateOptions{})
	}()

	sm.On("LoadSession", "TICKET-1").Return(&runner.SessionState{Name: "TICKET-1", Status: "running"}, nil)
	sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		return s.Status == "completed"
	})).Return(nil)

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
	assert.Equal(t, "/workspace", envMap["RECAC_HOST_WORKSPACE_PATH"], "RECAC_HOST_WORKSPACE_PATH should be propagated")
	// Note: Env Var still defaults to 20 via collectAgentEnvVars because host env is empty
	assert.Equal(t, "20", envMap["RECAC_MAX_ITERATIONS"], "RECAC_MAX_ITERATIONS env var should be default 20")

	// Check Flags
	cmdArgs := job.Spec.Template.Spec.Containers[0].Command
	foundMaxIter := false
	foundManagerFreq := false
	foundTaskMaxIter := false

	for i, arg := range cmdArgs {
		if arg == "--max-iterations" && i+1 < len(cmdArgs) && cmdArgs[i+1] == "42" {
			foundMaxIter = true
		}
		if arg == "--manager-frequency" && i+1 < len(cmdArgs) && cmdArgs[i+1] == "7" {
			foundManagerFreq = true
		}
		if arg == "--task-max-iterations" && i+1 < len(cmdArgs) && cmdArgs[i+1] == "12" {
			foundTaskMaxIter = true
		}
	}

	assert.True(t, foundMaxIter, "Command should contain --max-iterations 42")
	assert.True(t, foundManagerFreq, "Command should contain --manager-frequency 7")
	assert.True(t, foundTaskMaxIter, "Command should contain --task-max-iterations 12")

	sm.AssertExpectations(t)
}

func TestK8sSpawner_Spawn_Lifecycle(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sm := new(MockSessionManagerK8s)

	spawner := &K8sSpawner{
		Client:         clientset,
		Namespace:      "test-ns",
		Image:          "recac-agent:latest",
		AgentProvider:  "gemini",
		AgentModel:     "gemini-pro",
		PullPolicy:     corev1.PullAlways,
		Logger:         logger,
		SessionManager: sm,
	}

	item := WorkItem{
		ID:      "TASK-123",
		RepoURL: "https://github.com/example/repo",
		EnvVars: map[string]string{
			"CUSTOM_VAR": "value",
		},
	}

	t.Run("Create Success", func(t *testing.T) {
		sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
			return s.Status == "running"
		})).Return(nil)

		sm.On("LoadSession", "TASK-123").Return(&runner.SessionState{Name: "TASK-123", Status: "running"}, nil)
		sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
			return s.Status == "completed"
		})).Return(nil)

		go func() {
			for {
				_, err := clientset.BatchV1().Jobs("test-ns").Get(context.Background(), "recac-agent-task-123", metav1.GetOptions{})
				if err == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			job, _ := clientset.BatchV1().Jobs("test-ns").Get(context.Background(), "recac-agent-task-123", metav1.GetOptions{})
			job.Status.Succeeded = 1
			clientset.BatchV1().Jobs("test-ns").Update(context.Background(), job, metav1.UpdateOptions{})
		}()

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
		// Clean up from previous run if any
		clientset.BatchV1().Jobs("test-ns").Delete(context.Background(), "recac-agent-task-123", metav1.DeleteOptions{})

		// Re-create failed job manually
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: "recac-agent-task-123",
			},
			Status: batchv1.JobStatus{
				Failed: 1,
			},
		}
		clientset.BatchV1().Jobs("test-ns").Create(context.Background(), job, metav1.CreateOptions{})

		// Spawn should detect failed job and delete it, then return error to retry next cycle
		err := spawner.Spawn(context.Background(), item)
		// Should return error indicating cleanup and retry
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cleaning up failed job")

		// Verify Job was deleted (or deletion requested)
		_, err = clientset.BatchV1().Jobs("test-ns").Get(context.Background(), "recac-agent-task-123", metav1.GetOptions{})
		assert.Error(t, err) // Should be deleted in fake clientset immediately usually
	})

	t.Run("Wait For Job Timeout Error", func(t *testing.T) {
		// Clean up from previous run if any
		clientset.BatchV1().Jobs("test-ns").Delete(context.Background(), "recac-agent-task-123", metav1.DeleteOptions{})

		sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
			return s.Status == "running"
		})).Return(nil)
		sm.On("LoadSession", "TASK-123").Return(&runner.SessionState{Name: "TASK-123", Status: "running"}, nil)
		sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
			return s.Status == "error" && strings.Contains(s.Error, "context deadline exceeded")
		})).Return(nil)

		// Create a context with a very short timeout so waitErr will be context deadline exceeded
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := spawner.Spawn(ctx, item)
		assert.ErrorIs(t, err, context.DeadlineExceeded)

		// Job should be deleted (canceled)
		_, err = clientset.BatchV1().Jobs("test-ns").Get(context.Background(), "recac-agent-task-123", metav1.GetOptions{})
		assert.Error(t, err)
	})

	t.Run("Wait For Job Failed Error", func(t *testing.T) {
		// Clean up from previous run if any
		clientset.BatchV1().Jobs("test-ns").Delete(context.Background(), "recac-agent-task-123", metav1.DeleteOptions{})

		sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
			return s.Status == "running"
		})).Return(nil)
		sm.On("LoadSession", "TASK-123").Return(&runner.SessionState{Name: "TASK-123", Status: "running"}, nil)
		sm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
			return s.Status == "error" && strings.Contains(s.Error, "job failed with 1 failed pods")
		})).Return(nil)

		go func() {
			for {
				job, err := clientset.BatchV1().Jobs("test-ns").Get(context.Background(), "recac-agent-task-123", metav1.GetOptions{})
				if err == nil {
					job.Status.Failed = 1
					clientset.BatchV1().Jobs("test-ns").Update(context.Background(), job, metav1.UpdateOptions{})
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()

		err := spawner.Spawn(context.Background(), item)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "job failed with 1 failed pods")
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
	sm := new(MockSessionManagerK8s)
	spawner := &K8sSpawner{
		Client:         clientset,
		Namespace:      "test-ns",
		Logger:         logger,
		SessionManager: sm,
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

	// Test Cancel NotFound
	err = spawner.Cancel(ctx, "NON-EXISTENT")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestK8sSpawner_GetLogs(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sm := new(MockSessionManagerK8s)
	spawner := &K8sSpawner{
		Client:         clientset,
		Namespace:      "default",
		Logger:         logger,
		SessionManager: sm,
	}

	ctx := context.Background()
	jobID := "LOG-JOB-1"
	podName := "recac-agent-log-job-1-pod"

	// 1. No Pods
	_, err := spawner.GetLogs(ctx, jobID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no active pods found")

	// 2. Pod Exists
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: "default",
			Labels:    map[string]string{"work-item": jobID},
		},
	}
	_, err = clientset.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Attempt GetLogs.
	// Fake client implementation of GetLogs returns a dummy request which might succeed with empty stream.
	stream, err := spawner.GetLogs(ctx, jobID)
	assert.NoError(t, err)
	if stream != nil {
		stream.Close()
	}
}
