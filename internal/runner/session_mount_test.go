package runner

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"recac/internal/agent"
	"recac/internal/docker"
)

// MockAgent for testing
type MockAgentForMount struct{}

func (m *MockAgentForMount) Send(ctx context.Context, prompt string) (string, error) {
	return "mock response", nil
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

	// Step 2: Setup mock Docker client that simulates container execution
	dockerClient, mock := docker.NewMockClient()
	containerID := "test-container-123"
	
	// Track the workspace path that was mounted
	var mountedWorkspace string
	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		// Verify the mount configuration
		if len(hostConfig.Binds) == 0 {
			t.Error("Expected at least one bind mount, got none")
		}
		
		// Extract the workspace path from the bind mount
		// Format: "/host/path:/workspace"
		bindMount := hostConfig.Binds[0]
		parts := strings.Split(bindMount, ":")
		if len(parts) != 2 {
			t.Errorf("Expected bind mount format 'host:container', got %s", bindMount)
		}
		if parts[1] != "/workspace" {
			t.Errorf("Expected container mount point to be /workspace, got %s", parts[1])
		}
		mountedWorkspace = parts[0]
		
		// Verify working directory
		if config.WorkingDir != "/workspace" {
			t.Errorf("Expected WorkingDir to be /workspace, got %s", config.WorkingDir)
		}
		
		return container.CreateResponse{ID: containerID}, nil
	}

	// Mock the exec command to return the actual file listing from the workspace
	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		// Verify the command is 'ls /workspace'
		if len(config.Cmd) < 2 || config.Cmd[0] != "ls" || config.Cmd[1] != "/workspace" {
			t.Errorf("Expected command 'ls /workspace', got %v", config.Cmd)
		}
		return types.IDResponse{ID: "exec-id-123"}, nil
	}

	// Simulate ls output based on actual workspace contents
	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		// Read actual files from the workspace directory
		entries, err := os.ReadDir(tmpDir)
		if err != nil {
			t.Fatalf("Failed to read workspace directory: %v", err)
		}
		
		var fileList []string
		for _, entry := range entries {
			fileList = append(fileList, entry.Name())
		}
		
		// Create Docker-style multiplexed output
		output := strings.Join(fileList, "\n") + "\n"
		var buf bytes.Buffer
		
		// Stdout header (type 1). The last 4 bytes are BigEndian size.
		header := [8]byte{1, 0, 0, 0, 0, 0, 0, byte(len(output))}
		buf.Write(header[:])
		buf.Write([]byte(output))
		
		return types.HijackedResponse{
			Conn:   &fakeConn{},
			Reader: bufio.NewReader(&buf),
		}, nil
	}

	// Step 3: Start session and execute ls command
	session := NewSession(dockerClient, &MockAgentForMount{}, tmpDir, "alpine:latest")
	
	ctx := context.Background()
	if err := session.Start(ctx); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Execute ls /workspace in the container
	output, err := dockerClient.Exec(ctx, containerID, []string{"ls", "/workspace"})
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
	
	// Verify mounted workspace path
	if mountedWorkspace != tmpDir {
		t.Errorf("Expected workspace path %s to be mounted, but got %s", tmpDir, mountedWorkspace)
	}
	
	t.Logf("Successfully verified workspace mounting:")
	t.Logf("  Workspace path: %s", tmpDir)
	t.Logf("  Mounted as: %s:/workspace", mountedWorkspace)
	t.Logf("  Container files: %v", outputLines)
}
