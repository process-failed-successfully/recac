package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestK8sSpawner_Spawn_CommandGeneration(t *testing.T) {
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

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
	}

	// Execute
	err := spawner.Spawn(context.Background(), item)
	assert.NoError(t, err)

	// Verify
	jobName := "recac-agent-ticket-1"
	job, err := fakeClient.BatchV1().Jobs("default").Get(context.Background(), jobName, metav1.GetOptions{})
	assert.NoError(t, err)

	args := job.Spec.Template.Spec.Containers[0].Args
	assert.Len(t, args, 1)
	command := args[0]

	// Assert: No git config hack (This should FAIL currently)
	if assert.NotContains(t, command, "git config --global url") {
		// If it doesn't contain it, good.
	} else {
		t.Log("Command contains git config hack as expected (pre-fix)")
	}

	if assert.NotContains(t, command, "if [ -n \"$GITHUB_TOKEN\" ]; then") {
		// If it doesn't contain it, good.
	} else {
		t.Log("Command contains if block as expected (pre-fix)")
	}

	// Assert: Uses proper flags including --verbose (This should FAIL currently)
	assert.Contains(t, command, "--verbose", "Should contain --verbose")
	assert.Contains(t, command, "--jira \"TICKET-1\"", "Should contain --jira")

	// Check that it starts directly with recac-agent (or shell wrapper that calls it directly)
	// The current implementation uses /bin/sh -c "string", so args[0] is the command string.
	// We want to ensure it doesn't have the "if ... fi" block.
	assert.NotContains(t, command, "insteadOf", "Should not contain insteadOf hack")
}
