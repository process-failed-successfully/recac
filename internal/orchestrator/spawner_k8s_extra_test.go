package orchestrator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestNewK8sSpawner_Fail(t *testing.T) {
	// Ensure we are not in a cluster and have no valid kubeconfig
	t.Setenv("KUBERNETES_SERVICE_HOST", "") // Ensure InClusterConfig fails

	// Create a dummy file that is NOT a valid kubeconfig
	tmpDir := t.TempDir()
	invalidConfig := filepath.Join(tmpDir, "invalid_config")
	os.WriteFile(invalidConfig, []byte("invalid yaml"), 0644)

	t.Setenv("KUBECONFIG", invalidConfig)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner, err := NewK8sSpawner(logger, "img", "", "", "", corev1.PullAlways)

	// It should fail to load config
	assert.Error(t, err)
	assert.Nil(t, spawner)
}

func TestK8sSpawner_Spawn_JobExists(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := &K8sSpawner{
		Client: clientset,
		Namespace: "default",
		Logger: logger,
	}
	item := WorkItem{ID: "EXISTING"}

	// Case 1: Active
	jobActive := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "recac-agent-existing", Namespace: "default"},
		Status: batchv1.JobStatus{Active: 1},
	}
	clientset.Tracker().Add(jobActive)

	err := spawner.Spawn(context.Background(), item)
	assert.NoError(t, err) // Should return nil

	// Case 2: Succeeded
	itemSuc := WorkItem{ID: "SUCCEEDED"}
	jobSuc := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "recac-agent-succeeded", Namespace: "default"},
		Status: batchv1.JobStatus{Succeeded: 1},
	}
	clientset.Tracker().Add(jobSuc)

	err = spawner.Spawn(context.Background(), itemSuc)
	assert.NoError(t, err) // Should return nil
}

func TestK8sSpawner_Spawn_GetError(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	// Mock Get to return error
	clientset.PrependReactor("get", "jobs", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("db connection error")
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := &K8sSpawner{
		Client: clientset,
		Namespace: "default",
		Logger: logger,
	}
	item := WorkItem{ID: "ERROR-GET"}

	err := spawner.Spawn(context.Background(), item)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check for existing job")
}

func TestK8sSpawner_Spawn_CreateError(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	// Mock Create to return error
	clientset.PrependReactor("create", "jobs", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("quota exceeded")
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := &K8sSpawner{
		Client: clientset,
		Namespace: "default",
		Logger: logger,
	}
	item := WorkItem{ID: "ERROR-CREATE"}

	err := spawner.Spawn(context.Background(), item)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create job")
}

func TestK8sSpawner_Spawn_DeleteError(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	// Add failed job
	jobFailed := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "recac-agent-failed-del", Namespace: "default"},
		Status: batchv1.JobStatus{Failed: 1},
	}
	clientset.Tracker().Add(jobFailed)

	// Mock Delete to fail
	clientset.PrependReactor("delete", "jobs", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("delete forbidden")
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := &K8sSpawner{
		Client: clientset,
		Namespace: "default",
		Logger: logger,
	}
	item := WorkItem{ID: "FAILED-DEL"}

	err := spawner.Spawn(context.Background(), item)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete failed job")
}

func TestBoolPtr(t *testing.T) {
	b := boolPtr(true)
	assert.NotNil(t, b)
	assert.True(t, *b)

	b2 := boolPtr(false)
	assert.False(t, *b2)
}
