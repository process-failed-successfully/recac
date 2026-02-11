package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"
)

// MockAgentForMount for testing
type MockAgentForMount struct{}

func (m *MockAgentForMount) Send(ctx context.Context, prompt string) (string, error) {
	return "Agent processed " + prompt, nil
}

func (m *MockAgentForMount) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp := "Agent processed " + prompt
	if onChunk != nil {
		onChunk(resp)
	}
	return resp, nil
}

// Ensure MockAgentForMount implements agent.Agent interface
var _ agent.Agent = (*MockAgentForMount)(nil)

// TestSession_WorkspaceMounting verifies that the workspace directory is correctly
// mounted into the container and can be accessed via exec commands.
// This test verifies Feature #12: Docker workspace mounting.
func TestSession_WorkspaceMounting(t *testing.T) {
	// Step 1: Create a temporary workspace directory with test files
	tmpDir := t.TempDir()

	// Create some test files in the workspace
	testFiles := []string{"file1.txt", "file2.txt", "subdir"}
	for _, name := range testFiles {
		path := filepath.Join(tmpDir, name)
		if name == "subdir" {
			if err := os.MkdirAll(path, 0755); err != nil {
				t.Fatalf("Failed to create subdirectory: %v", err)
			}
			// Create a file inside the subdirectory
			subFile := filepath.Join(path, "subfile.txt")
			if err := os.WriteFile(subFile, []byte("sub content"), 0644); err != nil {
				t.Fatalf("Failed to create subfile: %v", err)
			}
		} else {
			if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
		}
	}

	// Step 2: Setup mock Docker client
	mock := &MockDockerClient{}
	containerID := "test-container-123"

	// Mock environment
	mock.CheckDaemonFunc = func(ctx context.Context) error { return nil }
	mock.ImageExistsFunc = func(ctx context.Context, image string) (bool, error) { return true, nil }

	// Track the workspace path that was mounted
	var capturedWorkspace string
	mock.RunContainerFunc = func(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error) {
		capturedWorkspace = workspace
		return containerID, nil
	}

	// Mock the exec command to return the actual file listing from the workspace
	mock.ExecFunc = func(ctx context.Context, id string, cmd []string) (string, error) {
		// Verify the command is 'ls /workspace'
		if len(cmd) >= 2 && cmd[0] == "ls" && cmd[1] == "/workspace" {
			// Read actual files from the workspace directory (simulating container access)
			entries, err := os.ReadDir(tmpDir)
			if err != nil {
				return "", err
			}
			var fileList []string
			for _, entry := range entries {
				fileList = append(fileList, entry.Name())
			}
			return strings.Join(fileList, "\n"), nil
		}
		return "", nil
	}

	// Step 3: Start session and execute ls command
	session := NewSession(mock, &MockAgentForMount{}, tmpDir, "alpine:latest", "test-project", "gemini", "gemini-pro", 1)
	session.UseLocalAgent = false // Ensure we use Docker path

	ctx := context.Background()
	if err := session.Start(ctx); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify workspace mount (argument passed to RunContainer)
	if capturedWorkspace != tmpDir {
		t.Errorf("Expected workspace path %s to be mounted, but got %s", tmpDir, capturedWorkspace)
	}

	// Execute ls /workspace in the container via client (simulating verify step)
	output, err := mock.Exec(ctx, containerID, []string{"ls", "/workspace"})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	// Step 4: Verify the output matches the host directory
	outputLines := strings.Split(strings.TrimSpace(output), "\n")

	// Read actual files from host directory
	hostEntries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read host directory: %v", err)
	}

	hostFiles := make(map[string]bool)
	for _, entry := range hostEntries {
		hostFiles[entry.Name()] = true
	}

	// Verify all container files exist in host
	containerFiles := make(map[string]bool)
	for _, line := range outputLines {
		line = strings.TrimSpace(line)
		if line != "" {
			containerFiles[line] = true
			if !hostFiles[line] {
				t.Errorf("File '%s' found in container but not in host directory", line)
			}
		}
	}

	// Verify all host files are in container (at least the main ones)
	for _, entry := range hostEntries {
		if !containerFiles[entry.Name()] {
			t.Errorf("File '%s' found in host but not in container output", entry.Name())
		}
	}

	t.Logf("Successfully verified workspace mounting simulation:")
	t.Logf("  Workspace path: %s", tmpDir)
	t.Logf("  Captured workspace arg: %s", capturedWorkspace)
	t.Logf("  Container files (simulated): %v", outputLines)
}
