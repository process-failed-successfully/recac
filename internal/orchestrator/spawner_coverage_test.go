package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestK8sSpawner_Helpers(t *testing.T) {
	// Test boolPtr
	b := true
	ptr := boolPtr(b)
	if *ptr != b {
		t.Error("boolPtr failed")
	}

	// Test extractRepoPath
	url := "https://github.com/org/repo"
	path := extractRepoPath(url)
	if path != "org/repo" {
		t.Errorf("extractRepoPath failed, got %s", path)
	}

	// Test sanitizeK8sName
	name := "TEST-Project_123"
	safe := sanitizeK8sName(name)
	// Lowercase and replace non-alphanumeric with -
	// test-project-123
	if safe != "test-project-123" {
		t.Errorf("sanitizeK8sName failed, got %s", safe)
	}
}

func TestK8sSpawner_Cleanup(t *testing.T) {
	// Cleanup is empty but we should cover it
	s := &K8sSpawner{}
	err := s.Cleanup(context.Background(), WorkItem{})
	if err != nil {
		t.Error("K8sSpawner.Cleanup should return nil")
	}
}

func TestDockerSpawner_Cleanup(t *testing.T) {
	// Cleanup is empty but we should cover it
	s := &DockerSpawner{}
	err := s.Cleanup(context.Background(), WorkItem{})
	if err != nil {
		t.Error("DockerSpawner.Cleanup should return nil")
	}
}

func TestNewK8sSpawner_Fail(t *testing.T) {
	// Force fail by setting KUBECONFIG to invalid path
	os.Setenv("KUBECONFIG", "/invalid/path/to/kubeconfig")
	defer os.Unsetenv("KUBECONFIG")
	// Also ensure HOME is set so it doesn't fallback to valid home default if KUBECONFIG is not set (Wait, code checks HomeDir() first? No.
	// Code:
	// if home := homedir.HomeDir(); home != "" { kubeconfig = ... } else { kubeconfig = os.Getenv("KUBECONFIG") }
	// Wait, if home is set, it ignores KUBECONFIG env var in fallback?
	// The code in NewK8sSpawner:
	// if home := homedir.HomeDir(); home != "" {
	//     kubeconfig = filepath.Join(home, ".kube", "config")
	// } else {
	//     kubeconfig = os.Getenv("KUBECONFIG")
	// }
	// This looks like a bug or simplistic logic in the code (it prioritizes default home location over env var if home exists?).
	// Standard clientcmd.BuildConfigFromFlags uses KUBECONFIG env var if set.
	// But `NewK8sSpawner` logic constructs `kubeconfig` string manually.

	// If I mock HOME to empty, it goes to else.

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", "")
	defer os.Setenv("HOME", origHome)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := NewK8sSpawner(logger, "image", "ns", "provider", "model", "")
	assert.Error(t, err)
}
