package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"recac/internal/runner"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDockerSpawner_Spawn_DryRun(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	mockPoller := new(MockPoller)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := NewDockerSpawner(logger, mockDocker, "test-image", "test-proj", mockPoller, "provider", "model", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "DRY-RUN-TEST",
		RepoURL: "https://github.com/test/repo",
		DryRun:  true,
	}

	ctx := context.Background()

	// Mock expectations
	mockDocker.On("RunContainerWithLabels", ctx, "test-image", mock.AnythingOfType("string"), mock.Anything, mock.Anything, "", mock.Anything).Return("container123", nil)

	mockSM.On("SaveSession", mock.AnythingOfType("*runner.SessionState")).Return(nil)
	mockSM.On("LoadSession", "DRY-RUN-TEST").Return(&runner.SessionState{}, nil)

	// Verify Exec command includes --dry-run
	mockDocker.On("Exec", mock.Anything, "container123", mock.MatchedBy(func(cmd []string) bool {
		cmdStr := cmd[2]
		return strings.Contains(cmdStr, "--dry-run")
	})).Return("output", nil)

	mockGit.On("CurrentCommitSHA", mock.AnythingOfType("string")).Return("endsha", nil).Once()

	err := spawner.Spawn(ctx, item)

	assert.NoError(t, err)

	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
}

func TestK8sSpawner_Spawn_DryRun(t *testing.T) {
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

	item := WorkItem{
		ID:      "DRY-RUN-K8S",
		RepoURL: "https://github.com/test/repo",
		DryRun:  true,
	}

	err := spawner.Spawn(context.Background(), item)
	assert.NoError(t, err)

	// Verify Job created with --dry-run arg
	jobName := "recac-agent-dry-run-k8s"
	job, err := fakeClient.BatchV1().Jobs("default").Get(context.Background(), jobName, metav1.GetOptions{})
	assert.NoError(t, err)

	// Check command args
	// K8sSpawner puts the command in Args[0] wrapped in sh -c
	args := job.Spec.Template.Spec.Containers[0].Args[0]
	assert.Contains(t, args, "--dry-run", "Command should contain --dry-run flag")
}
