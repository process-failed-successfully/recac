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
		spawner := &K8sSpawner{
			Client:        k8sClient,
			Namespace:     "ns",
			Image:         "img",
			AgentProvider: "prov",
			AgentModel:    "mod",
			PullPolicy:    corev1.PullAlways,
			Logger:        logger,
		}

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
		spawner := NewDockerSpawner(logger, mockDocker, "img", "proj", nil, "prov", "mod", mockSM)

		// Use a mock GitClient that does nothing
		mockGit := new(MockGitClient)
		mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)
		spawner.GitClient = mockGit

		// Expectations
		mockDocker.On("RunContainer", mock.Anything, "img", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("cid", nil)
		mockSM.On("SaveSession", mock.Anything).Return(nil)
		mockSM.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

		capturedCmdChan := make(chan []string, 1)
		mockDocker.On("Exec", mock.Anything, "cid", mock.Anything).Run(func(args mock.Arguments) {
			capturedCmd := args.Get(2).([]string)
			capturedCmdChan <- capturedCmd
		}).Return("out", nil)

		err := spawner.Spawn(ctx, item)
		assert.NoError(t, err)

		var capturedCmd []string
		select {
		case capturedCmd = <-capturedCmdChan:
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for Exec")
		}

		cmdStr := capturedCmd[2]

		// Assertions
		assert.Contains(t, cmdStr, "export RECAC_MAX_ITERATIONS=50", "Docker should propagate RECAC_MAX_ITERATIONS")
		assert.Contains(t, cmdStr, "export RECAC_MANAGER_FREQUENCY=10m", "Docker should propagate RECAC_MANAGER_FREQUENCY")
		assert.Contains(t, cmdStr, "export RECAC_TASK_MAX_ITERATIONS=5", "Docker should propagate RECAC_TASK_MAX_ITERATIONS")
	})
}
