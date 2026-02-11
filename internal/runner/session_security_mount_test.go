package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"recac/internal/docker"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// MockAgentForSecurity implements agent.Agent
type MockAgentForSecurity struct{}
func (m *MockAgentForSecurity) Send(ctx context.Context, prompt string) (string, error) { return "", nil }
func (m *MockAgentForSecurity) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) { return "", nil }


// mockFileInfo implements os.FileInfo
type mockFileInfo struct {
	name  string
	isDir bool
}
func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() os.FileMode  {
    if m.isDir { return os.ModeDir | 0755 }
    return 0644
}
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() any           { return nil }


func TestSession_SensitiveMounts_ReadOnly(t *testing.T) {
	// Setup mock StatFunc instead of real FS
    mockHome := "/home/mockuser"

    mockStat := func(path string) (os.FileInfo, error) {
        // Allow sensitive paths
        sensitiveDirs := []string{".gemini", ".config", ".cursor", ".ssh"}
        for _, dir := range sensitiveDirs {
            target := filepath.Join(mockHome, dir)
            if path == target {
                return &mockFileInfo{name: dir, isDir: true}, nil
            }
        }
        return nil, os.ErrNotExist
    }

	// Setup mock Docker client
	client, mock := docker.NewMockClient()

	// We want to capture the binds
	var capturedBinds []string

	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		capturedBinds = hostConfig.Binds
		return container.CreateResponse{ID: "test-id"}, nil
	}

	// Mock ImageList to return our image so ImageExists returns true
	mock.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
		return []image.Summary{{RepoTags: []string{"alpine:latest"}}}, nil
	}

	mock.ContainerStartFunc = func(ctx context.Context, containerID string, options container.StartOptions) error { return nil }
	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		return types.IDResponse{ID: "exec-id"}, nil
	}

	// Use NewSessionWithConfig to avoid DB initialization issues
	session := NewSessionWithConfig("/tmp/workspace", "test-project", "mock", "mock-model", nil)
	session.Docker = client
	session.Agent = &MockAgentForSecurity{}
	session.Image = "alpine:latest"
	session.HomeDir = mockHome
    session.StatFunc = mockStat
    session.UseLocalAgent = false // Explicitly ensure Docker usage

	// Start session
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Session.Start failed: %v", err)
	}

	// Verify sensitive mounts
	sensitivePaths := []string{".ssh", ".config", ".gemini", ".cursor"}

	foundAny := false
	for _, path := range sensitivePaths {
		found := false
		for _, bind := range capturedBinds {
			// Check if bind contains the sensitive path
			// Note: bind format is /host/path:/container/path:ro
            // We check if it matches the expected bind string structure for robustness
            expectedBindPrefix := filepath.Join(mockHome, path)
			if strings.HasPrefix(bind, expectedBindPrefix) {
				found = true
				foundAny = true
				// Check if it is Read-Only
				if !strings.HasSuffix(bind, ":ro") {
					t.Errorf("Security Vulnerability: Sensitive path '%s' is mounted Read-Write! Bind: %s", path, bind)
				}
			}
		}
		if !found {
			t.Errorf("Error: Sensitive path '%s' not found in binds", path)
		}
	}

	if !foundAny {
		t.Error("No sensitive paths were found in binds.")
	}
}
