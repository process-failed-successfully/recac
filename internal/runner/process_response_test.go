package runner

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/notify"
	"testing"
)


func TestSession_ProcessResponse_NoCommands(t *testing.T) {
	s := &Session{
		Docker:   &MockDockerClient{},
		Logger:   slog.Default(),
		Notifier: notify.NewManager(func(string, ...interface{}) {}),
		Project:  "test-project",
	}

	output, err := s.ProcessResponse(context.Background(), "Just some text")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if output != "" {
		t.Errorf("Expected empty output, got %s", output)
	}
}

func TestSession_ProcessResponse_WithCommands(t *testing.T) {
	mockDocker := &MockDockerClient{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			return "Success", nil
		},
	}

	// Create a temporary workspace for blocker file checks
	workspace := t.TempDir()

	// Create DB
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	s := &Session{
		Docker:    mockDocker,
		Workspace: workspace,
		DBStore:   store,
		Logger:    slog.Default(),
		Notifier:  notify.NewManager(func(string, ...interface{}) {}),
		Project:   "test-project",
	}

	response := "Here is code:\n```bash\necho hello\n```"
	output, err := s.ProcessResponse(context.Background(), response)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := "Command Output:\nSuccess\n"
	if output != expected {
		t.Errorf("Expected output containing 'Success', got %s", output)
	}
}

func TestSession_ProcessResponse_Blocker(t *testing.T) {
	mockDocker := &MockDockerClient{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			return "Success", nil
		},
	}

	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	s := &Session{
		Docker:    mockDocker,
		Workspace: workspace,
		DBStore:   store,
		Logger:    slog.Default(),
		Notifier:  notify.NewManager(func(string, ...interface{}) {}),
		Project:   "test-project",
	}

	// Test 1: DB Signal Blocker
	store.SetSignal("test-project", "BLOCKER", "I am stuck")
	_, err := s.ProcessResponse(context.Background(), "some commands")
	if err != ErrBlocker {
		t.Errorf("Expected ErrBlocker from DB signal, got %v", err)
	}

	// Test 2: File Blocker
	store.DeleteSignal("test-project", "BLOCKER") // Clear DB signal

	blockerFile := filepath.Join(workspace, "recac_blockers.txt")
	os.WriteFile(blockerFile, []byte("I am blocked by missing API key"), 0644)

	_, err = s.ProcessResponse(context.Background(), "some commands")
	if err != ErrBlocker {
		t.Errorf("Expected ErrBlocker from file, got %v", err)
	}

	// Test 3: False Positive Blocker
	os.WriteFile(blockerFile, []byte("No blockers found. All tests passed."), 0644)
	_, err = s.ProcessResponse(context.Background(), "some commands")
	if err != nil {
		t.Errorf("Expected no error for false positive blocker, got %v", err)
	}

	// Check if file was removed
	if _, err := os.Stat(blockerFile); !os.IsNotExist(err) {
		t.Errorf("Expected blocker file to be removed after false positive detection")
	}
}
