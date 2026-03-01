package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"os"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestSpawnerConsistency checks that both K8s and Docker spawners propagate the same configuration
// environment variables from the host.
func TestSpawnerConsistency_EnvPropagation(t *testing.T) {
	// Set up host environment
	os.Setenv("RECAC_MAX_ITERATIONS", "50")
	os.Setenv("RECAC_MANAGER_FREQUENCY", "10m")
	os.Setenv("RECAC_TASK_MAX_ITERATIONS", "5")
	defer func() {
		os.Unsetenv("RECAC_MAX_ITERATIONS")
		os.Unsetenv("RECAC_MANAGER_FREQUENCY")
		os.Unsetenv("RECAC_TASK_MAX_ITERATIONS")
	}()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	item := WorkItem{
		ID:      "TASK-CONSISTENCY",
		RepoURL: "https://github.com/example/repo",
	}

	// 1. Check K8s Spawner
	t.Run("K8sSpawner propagates all config vars", func(t *testing.T) {
		k8sClient := fake.NewSimpleClientset()
		mockSM := new(MockSessionManager)
		spawner := &K8sSpawner{
			Client:         k8sClient,
			Namespace:      "ns",
			Image:          "img",
			AgentProvider:  "prov",
			AgentModel:     "mod",
			PullPolicy:     corev1.PullAlways,
			Logger:         logger,
			SessionManager: mockSM,
		}

		mockSM.On("SaveSession", mock.Anything).Return(nil)
		mockSM.On("LoadSession", mock.Anything).Return(&runner.SessionState{Name: "TASK-CONSISTENCY", Status: "running"}, nil)

		// Simulate job completion for waitForJob
		go func() {
			for {
				_, err := k8sClient.BatchV1().Jobs("ns").Get(context.Background(), "recac-agent-task-consistency", metav1.GetOptions{})
				if err == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			job, _ := k8sClient.BatchV1().Jobs("ns").Get(context.Background(), "recac-agent-task-consistency", metav1.GetOptions{})
			job.Status.Succeeded = 1
			k8sClient.BatchV1().Jobs("ns").Update(context.Background(), job, metav1.UpdateOptions{})
		}()

		err := spawner.Spawn(ctx, item)
		assert.NoError(t, err)

		// Get the created job
		job, err := k8sClient.BatchV1().Jobs("ns").Get(ctx, "recac-agent-task-consistency", metav1.GetOptions{})
		assert.NoError(t, err)

		envVars := job.Spec.Template.Spec.Containers[0].Env
		envMap := make(map[string]string)
		for _, e := range envVars {
			envMap[e.Name] = e.Value
		}

		// Assertions
		assert.Equal(t, "50", envMap["RECAC_MAX_ITERATIONS"], "K8s should propagate RECAC_MAX_ITERATIONS")
		assert.Equal(t, "10m", envMap["RECAC_MANAGER_FREQUENCY"], "K8s should propagate RECAC_MANAGER_FREQUENCY")
		assert.Equal(t, "5", envMap["RECAC_TASK_MAX_ITERATIONS"], "K8s should propagate RECAC_TASK_MAX_ITERATIONS")

		// Check for duplicates in K8s (Env list)
		count := 0
		for _, e := range envVars {
			if e.Name == "RECAC_MAX_ITERATIONS" {
				count++
			}
		}
		assert.Equal(t, 1, count, "K8s should not have duplicate RECAC_MAX_ITERATIONS env vars")
	})

	// 2. Check Docker Spawner
	t.Run("DockerSpawner propagates all config vars", func(t *testing.T) {
		mockDocker := new(MockDockerClient)
		mockSM := new(MockSessionManager)
		spawner := NewDockerSpawner(logger, mockDocker, "img", "proj", nil, "prov", "mod", "Always", mockSM, 30, 5, 10)
		mockDocker.On("PullImage", mock.Anything, "img").Return(nil)

		// Use a mock GitClient that does nothing
		mockGit := new(MockGitClient)
		mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)
		spawner.GitClient = mockGit

		// Expectations
		capturedEnvChan := make(chan []string, 1)
		mockDocker.On("RunContainerWithLabels", mock.Anything, "img", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			env := args.Get(4).([]string) // env=4
			capturedEnvChan <- env
		}).Return("cid", nil)

		mockDocker.On("WaitContainer", mock.Anything, "cid").Return(int(0), nil)

		mockSM.On("SaveSession", mock.Anything).Return(nil)
		mockSM.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

		err := spawner.Spawn(ctx, item)
		assert.NoError(t, err)

		var capturedEnv []string
		select {
		case capturedEnv = <-capturedEnvChan:
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for RunContainerWithLabels")
		}

		// Assertions
		hasMaxIter := false
		hasFreq := false
		hasTaskIter := false

		for _, e := range capturedEnv {
			if e == "RECAC_MAX_ITERATIONS=50" {
				hasMaxIter = true
			}
			if e == "RECAC_MANAGER_FREQUENCY=10m" {
				hasFreq = true
			}
			if e == "RECAC_TASK_MAX_ITERATIONS=5" {
				hasTaskIter = true
			}
		}

		assert.True(t, hasMaxIter, "Docker should propagate RECAC_MAX_ITERATIONS")
		assert.True(t, hasFreq, "Docker should propagate RECAC_MANAGER_FREQUENCY")
		assert.True(t, hasTaskIter, "Docker should propagate RECAC_TASK_MAX_ITERATIONS")
	})
}

func TestSpawnerConsistency_LabelsAndGitConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	item := WorkItem{
		ID:      "TASK-CONSISTENCY",
		RepoURL: "https://github.com/example/repo",
	}

	// 1. Check K8s Spawner Labels
	t.Run("K8sSpawner sets correct labels", func(t *testing.T) {
		k8sClient := fake.NewSimpleClientset()
		mockSM := new(MockSessionManager)
		spawner := &K8sSpawner{
			Client:         k8sClient,
			Namespace:      "ns",
			Image:          "img",
			AgentProvider:  "prov",
			AgentModel:     "mod",
			PullPolicy:     corev1.PullAlways,
			Logger:         logger,
			SessionManager: mockSM,
		}

		mockSM.On("SaveSession", mock.Anything).Return(nil)
		mockSM.On("LoadSession", mock.Anything).Return(&runner.SessionState{Name: "TASK-CONSISTENCY", Status: "running"}, nil)

		go func() {
			for {
				_, err := k8sClient.BatchV1().Jobs("ns").Get(context.Background(), "recac-agent-task-consistency", metav1.GetOptions{})
				if err == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			job, _ := k8sClient.BatchV1().Jobs("ns").Get(context.Background(), "recac-agent-task-consistency", metav1.GetOptions{})
			job.Status.Succeeded = 1
			k8sClient.BatchV1().Jobs("ns").Update(context.Background(), job, metav1.UpdateOptions{})
		}()

		err := spawner.Spawn(ctx, item)
		assert.NoError(t, err)

		// Get the created job
		job, err := k8sClient.BatchV1().Jobs("ns").Get(ctx, "recac-agent-task-consistency", metav1.GetOptions{})
		assert.NoError(t, err)

		// Check Job Labels
		assert.Equal(t, "recac-orchestrator", job.Labels["created-by"], "K8s Job should have created-by label")
		assert.Equal(t, "TASK-CONSISTENCY", job.Labels["work-item"], "K8s Job should have work-item label")

		// Check Pod Template Labels
		podLabels := job.Spec.Template.ObjectMeta.Labels
		assert.Equal(t, "recac-orchestrator", podLabels["created-by"], "K8s Pod should have created-by label")
		assert.Equal(t, "TASK-CONSISTENCY", podLabels["work-item"], "K8s Pod should have work-item label")
	})

	// 2. Check Docker Spawner Git Config Injection
	t.Run("DockerSpawner injects git config command", func(t *testing.T) {
		mockDocker := new(MockDockerClient)
		mockSM := new(MockSessionManager)
		spawner := NewDockerSpawner(logger, mockDocker, "img", "proj", nil, "prov", "mod", "Always", mockSM, 30, 5, 10)
		mockDocker.On("PullImage", mock.Anything, "img").Return(nil)

		// Use a MockGitClient
		mockGit := new(MockGitClient)
		mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)
		spawner.GitClient = mockGit

		// Expectations
		capturedCmdChan := make(chan []string, 1)
		mockDocker.On("RunContainerWithLabels",
			mock.Anything,
			"img",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything, // user
			mock.MatchedBy(func(labels map[string]string) bool {
				return labels["created-by"] == "recac-orchestrator" && labels["work-item"] == "TASK-CONSISTENCY"
			}),
		).Run(func(args mock.Arguments) {
			capturedCmd := args.Get(5).([]string) // cmd=5
			capturedCmdChan <- capturedCmd
		}).Return("cid", nil)

		mockDocker.On("WaitContainer", mock.Anything, "cid").Return(int(0), nil)

		mockSM.On("SaveSession", mock.Anything).Return(nil)
		mockSM.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

		err := spawner.Spawn(ctx, item)
		assert.NoError(t, err)

		var capturedCmd []string
		select {
		case capturedCmd = <-capturedCmdChan:
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for RunContainerWithLabels")
		}

		cmdStr := capturedCmd[2]

		// Assertions
		assert.Contains(t, cmdStr, "git config --global url", "Docker command should contain git config injection")
		assert.Contains(t, cmdStr, "x-oauth-basic@github.com", "Docker command should configure x-oauth-basic")
	})
}

func TestSpawnerConsistency_CommandArgs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	item := WorkItem{
		ID:      "TASK-CONSISTENCY-ARGS",
		RepoURL: "https://github.com/example/repo",
	}

	// 1. Check K8s Spawner Args
	t.Run("K8sSpawner sets correct command args", func(t *testing.T) {
		k8sClient := fake.NewSimpleClientset()
		mockSM := new(MockSessionManager)
		spawner := &K8sSpawner{
			Client:         k8sClient,
			Namespace:      "ns",
			Image:          "img",
			AgentProvider:  "prov",
			AgentModel:     "mod",
			PullPolicy:     corev1.PullAlways,
			Logger:         logger,
			SessionManager: mockSM,
		}

		mockSM.On("SaveSession", mock.Anything).Return(nil)
		mockSM.On("LoadSession", mock.Anything).Return(&runner.SessionState{Name: "TASK-CONSISTENCY-ARGS", Status: "running"}, nil)

		go func() {
			for {
				_, err := k8sClient.BatchV1().Jobs("ns").Get(context.Background(), "recac-agent-task-consistency-args", metav1.GetOptions{})
				if err == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			job, _ := k8sClient.BatchV1().Jobs("ns").Get(context.Background(), "recac-agent-task-consistency-args", metav1.GetOptions{})
			job.Status.Succeeded = 1
			k8sClient.BatchV1().Jobs("ns").Update(context.Background(), job, metav1.UpdateOptions{})
		}()

		err := spawner.Spawn(ctx, item)
		assert.NoError(t, err)

		// Get the created job
		job, err := k8sClient.BatchV1().Jobs("ns").Get(ctx, "recac-agent-task-consistency-args", metav1.GetOptions{})
		assert.NoError(t, err)

		// Check Command
		cmd := job.Spec.Template.Spec.Containers[0].Args[0]
		assert.Contains(t, cmd, "--verbose", "K8s command should contain --verbose")
		assert.Contains(t, cmd, "--allow-dirty", "K8s command should contain --allow-dirty")
		assert.Contains(t, cmd, "recac-agent --jira", "K8s command should invoke recac-agent")
	})

	// 2. Check Docker Spawner Args
	t.Run("DockerSpawner sets correct command args", func(t *testing.T) {
		mockDocker := new(MockDockerClient)
		mockSM := new(MockSessionManager)
		spawner := NewDockerSpawner(logger, mockDocker, "img", "proj", nil, "prov", "mod", "Always", mockSM, 30, 5, 10)
		mockDocker.On("PullImage", mock.Anything, "img").Return(nil)

		// Use a MockGitClient
		mockGit := new(MockGitClient)
		mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)
		spawner.GitClient = mockGit

		// Expectations
		capturedCmdChan := make(chan []string, 1)
		mockDocker.On("RunContainerWithLabels", mock.Anything, "img", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			capturedCmd := args.Get(5).([]string) // cmd=5
			capturedCmdChan <- capturedCmd
		}).Return("cid", nil)

		mockDocker.On("WaitContainer", mock.Anything, "cid").Return(int(0), nil)

		mockSM.On("SaveSession", mock.Anything).Return(nil)
		mockSM.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

		err := spawner.Spawn(ctx, item)
		assert.NoError(t, err)

		var capturedCmd []string
		select {
		case capturedCmd = <-capturedCmdChan:
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for RunContainerWithLabels")
		}

		cmdStr := capturedCmd[2]

		// Assertions
		assert.Contains(t, cmdStr, "--verbose", "Docker command should contain --verbose")
		assert.Contains(t, cmdStr, "--allow-dirty", "Docker command should contain --allow-dirty")
		assert.Contains(t, cmdStr, "recac-agent --jira", "Docker command should invoke recac-agent without absolute path")
	})
}
