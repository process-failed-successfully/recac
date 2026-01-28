package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSpawnerConsistency_CommandFlags(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	item := WorkItem{
		ID:      "TASK-FLAGS",
		RepoURL: "https://github.com/example/repo",
	}

	t.Run("K8sSpawner passes --image flag", func(t *testing.T) {
		k8sClient := fake.NewSimpleClientset()
		spawner := &K8sSpawner{
			Client:        k8sClient,
			Namespace:     "ns",
			Image:         "my-custom-image:latest",
			AgentProvider: "prov",
			AgentModel:    "mod",
			PullPolicy:    corev1.PullAlways,
			Logger:        logger,
		}

		err := spawner.Spawn(ctx, item)
		assert.NoError(t, err)

		job, err := k8sClient.BatchV1().Jobs("ns").Get(ctx, "recac-agent-task-flags", metav1.GetOptions{})
		assert.NoError(t, err)

		// Check args in the command script
		// K8sSpawner wraps the command in /bin/sh -c "script"
		// The script should contain: recac-agent ... --image my-custom-image:latest ...
		script := job.Spec.Template.Spec.Containers[0].Args[0]
		assert.Contains(t, script, "--image my-custom-image:latest", "K8s command script should contain --image flag")
	})

	t.Run("DockerSpawner passes --image flag", func(t *testing.T) {
		mockDocker := new(MockDockerClient)
		mockSM := new(MockSessionManager)
		spawner := NewDockerSpawner(logger, mockDocker, "my-custom-image:latest", "proj", nil, "prov", "mod", mockSM)

		// Mock expectations
		mockDocker.On("RunContainer", mock.Anything, "my-custom-image:latest", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("cid", nil)
		mockSM.On("SaveSession", mock.Anything).Return(nil)
		mockSM.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

		capturedCmdChan := make(chan []string, 1)
		mockDocker.On("Exec", mock.Anything, "cid", mock.Anything).Run(func(args mock.Arguments) {
			capturedCmd := args.Get(2).([]string)
			capturedCmdChan <- capturedCmd
		}).Return("out", nil)

		// Use mock GitClient to avoid real git calls
		mockGit := new(MockGitClient)
		mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)
		spawner.GitClient = mockGit

		err := spawner.Spawn(ctx, item)
		assert.NoError(t, err)

		var capturedCmd []string
		select {
		case capturedCmd = <-capturedCmdChan:
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for Exec")
		}

		cmdStr := capturedCmd[2] // /bin/sh -c <cmdStr>
		assert.Contains(t, cmdStr, "--image my-custom-image:latest", "Docker command string should contain --image flag")
	})

	t.Run("DockerSpawner injects git config for GITHUB_TOKEN", func(t *testing.T) {
		mockDocker := new(MockDockerClient)
		mockSM := new(MockSessionManager)
		spawner := NewDockerSpawner(logger, mockDocker, "img", "proj", nil, "prov", "mod", mockSM)

		// Use mock GitClient
		mockGit := new(MockGitClient)
		mockGit.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)
		spawner.GitClient = mockGit

		// Mock expectations
		mockDocker.On("RunContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("cid", nil)
		mockSM.On("SaveSession", mock.Anything).Return(nil)
		mockSM.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

		capturedCmdChan := make(chan []string, 1)
		mockDocker.On("Exec", mock.Anything, "cid", mock.Anything).Run(func(args mock.Arguments) {
			capturedCmd := args.Get(2).([]string)
			capturedCmdChan <- capturedCmd
		}).Return("out", nil)

		// Simulate GITHUB_TOKEN in env (use space to ensure quoting)
		t.Setenv("GITHUB_TOKEN", "gh_token 123")

		err := spawner.Spawn(ctx, item)
		assert.NoError(t, err)

		var capturedCmd []string
		select {
		case capturedCmd = <-capturedCmdChan:
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for Exec")
		}

		cmdStr := capturedCmd[2]
		// We expect the git config command to be present in the shell command chain
		assert.Contains(t, cmdStr, "git config --global url", "Docker command should configure git global url for GITHUB_TOKEN")

		// Verify that the token is exported in the shell command
		assert.Contains(t, cmdStr, "export GITHUB_TOKEN='gh_token 123'", "Docker command should export the token (quoted)")

		// Verify that the git config uses the shell variable for interpolation
		// We expect literal ${GITHUB_TOKEN} in the git config part
		assert.Contains(t, cmdStr, "${GITHUB_TOKEN}", "Docker command should use shell interpolation for the token in git config")
	})
}
