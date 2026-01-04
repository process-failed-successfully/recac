package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestSession_GitIgnoreCreation(t *testing.T) {
	workspace, _ := os.MkdirTemp("", "recac-gitignore-test-*")
	defer os.RemoveAll(workspace)

	// Mock Docker client that does nothing for Exec
	mockDocker := &mockDockerClient{}

	s := &Session{
		Workspace:   workspace,
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      telemetry.NewLogger(true, ""),
	}

	viper.Set("git_user_email", "test@example.com")
	viper.Set("git_user_name", "Test User")

	err := s.bootstrapGit(context.Background())
	if err != nil {
		t.Fatalf("bootstrapGit failed: %v", err)
	}

	gitignorePath := filepath.Join(workspace, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		t.Fatal(".gitignore was not created")
	}

	content, _ := os.ReadFile(gitignorePath)
	if !strings.Contains(string(content), ".recac.db") {
		t.Errorf("Expected .recac.db to be ignored, but not found in .gitignore:\n%s", string(content))
	}
	if !strings.Contains(string(content), ".cache/") {
		t.Errorf("Expected .cache/ to be ignored, but not found in .gitignore:\n%s", string(content))
	}
}

func TestSession_GitIgnoreFunctional(t *testing.T) {
	workspace, _ := os.MkdirTemp("", "recac-gitignore-func-*")
	defer os.RemoveAll(workspace)

	// Init git repo
	exec.Command("git", "init", workspace).Run()
	exec.Command("git", "-C", workspace, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", workspace, "config", "user.name", "Test User").Run()

	mockDocker := &mockDockerClient{}
	s := &Session{
		Workspace:   workspace,
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      telemetry.NewLogger(true, ""),
	}

	viper.Set("git_user_email", "test@example.com")
	viper.Set("git_user_name", "Test User")

	s.bootstrapGit(context.Background())

	// Create a dummy file that should be ignored
	dbPath := filepath.Join(workspace, ".recac.db")
	os.WriteFile(dbPath, []byte("dummy db content"), 0644)

	// Create a source file that should NOT be ignored
	srcPath := filepath.Join(workspace, "main.go")
	os.WriteFile(srcPath, []byte("package main"), 0644)

	// git add .
	exec.Command("git", "-C", workspace, "add", ".").Run()

	// Check what is staged
	out, _ := exec.Command("git", "-C", workspace, "ls-files").CombinedOutput()
	files := string(out)

	if !strings.Contains(files, "main.go") {
		t.Errorf("main.go should be tracked, but not found in ls-files:\n%s", files)
	}
	if strings.Contains(files, ".recac.db") {
		t.Errorf(".recac.db should NOT be tracked, but found in ls-files:\n%s", files)
	}
}

// Minimal mock for testing
type mockDockerClient struct {
	DockerClient
}

func (m *mockDockerClient) ExecAsUser(ctx context.Context, id, user string, cmd []string) (string, error) {
	return "", nil
}
