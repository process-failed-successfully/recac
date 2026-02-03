package docker

import (
	"bufio"
	"context"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// MockAPI implements APIClient for testing and mock execution.
type MockAPI struct {
	PingFunc                 func(ctx context.Context) (types.Ping, error)
	ServerVersionFunc        func(ctx context.Context) (types.Version, error)
	ImageListFunc            func(ctx context.Context, options image.ListOptions) ([]image.Summary, error)
	ImagePullFunc            func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ImageBuildFunc           func(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error)
	ContainerCreateFunc      func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error)
	ContainerStartFunc       func(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerExecCreateFunc  func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error)
	ContainerExecAttachFunc  func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error)
	ContainerExecInspectFunc func(ctx context.Context, execID string) (container.ExecInspect, error)
	ContainerStopFunc        func(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemoveFunc      func(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerListFunc        func(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	ContainerKillFunc        func(ctx context.Context, containerID, signal string) error
	CloseFunc                func() error

	// State for simple filesystem simulation
	filesMu sync.Mutex
	files   map[string]string
	execCmd map[string][]string // Map execID to command
}

func (m *MockAPI) Ping(ctx context.Context) (types.Ping, error) {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return types.Ping{}, nil
}

func (m *MockAPI) ServerVersion(ctx context.Context) (types.Version, error) {
	if m.ServerVersionFunc != nil {
		return m.ServerVersionFunc(ctx)
	}
	return types.Version{
		Version:    "mock-docker-20.10.7",
		APIVersion: "1.41",
		Os:         "linux",
		Arch:       "amd64",
	}, nil
}

func (m *MockAPI) ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
	if m.ImageListFunc != nil {
		return m.ImageListFunc(ctx, options)
	}
	// Return mock images including ubuntu:latest
	return []image.Summary{
		{
			ID:       "sha256:mockubuntu123",
			RepoTags: []string{"ubuntu:latest"},
		},
	}, nil
}

func (m *MockAPI) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	if m.ImagePullFunc != nil {
		return m.ImagePullFunc(ctx, ref, options)
	}
	// Return empty reader
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *MockAPI) ImageBuild(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error) {
	if m.ImageBuildFunc != nil {
		return m.ImageBuildFunc(ctx, buildContext, options)
	}
	// Return mock successful build response
	mockStream := `{"stream":"Step 1/2 : FROM alpine\n"}
{"stream":" ---> abc123def456\n"}
{"stream":"Step 2/2 : RUN echo hello\n"}
{"stream":" ---> Running in container123\n"}
{"aux":{"ID":"sha256:mockimageid123456789"}}
{"stream":"Successfully built mockimageid123456789\n"}`
	return types.ImageBuildResponse{
		Body: io.NopCloser(strings.NewReader(mockStream)),
	}, nil
}

func (m *MockAPI) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
	if m.ContainerCreateFunc != nil {
		return m.ContainerCreateFunc(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{ID: "mock-container-id"}, nil
}

func (m *MockAPI) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	if m.ContainerStartFunc != nil {
		return m.ContainerStartFunc(ctx, containerID, options)
	}
	return nil
}

func (m *MockAPI) ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
	if m.ContainerExecCreateFunc != nil {
		return m.ContainerExecCreateFunc(ctx, container, config)
	}

	// Store command for simulation
	execID := "mock-exec-id"
	// Generate unique ID if needed, but for now fixed is fine as we usually run sequentially or can append random
	// A simple counter would be better if parallel

	m.filesMu.Lock()
	defer m.filesMu.Unlock()
	if m.execCmd == nil {
		m.execCmd = make(map[string][]string)
	}
	// Simple unique ID generation
	// But `docker.Client.Exec` logic creates a new exec each time.
	// We'll just assume sequential for the basic test case or overwrite.
	// Actually, `Exec` calls `ExecCreate` then `ExecAttach`.
	// We need to persist the command to `ExecAttach`.
	m.execCmd[execID] = config.Cmd

	return types.IDResponse{ID: execID}, nil
}

func (m *MockAPI) ContainerExecAttach(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
	if m.ContainerExecAttachFunc != nil {
		return m.ContainerExecAttachFunc(ctx, execID, config)
	}

	m.filesMu.Lock()
	if m.files == nil {
		m.files = make(map[string]string)
	}
	if m.execCmd == nil {
		m.execCmd = make(map[string][]string)
	}
	cmd := m.execCmd[execID]
	m.filesMu.Unlock()

	var output string

	if len(cmd) >= 3 && cmd[0] == "/bin/bash" && cmd[1] == "-c" {
		script := cmd[2]

		// Simple Write Simulation: echo 'content' > file
		// Note: This is fragile parsing but sufficient for the specific "echo > feature_list.json" case
		if strings.HasPrefix(strings.TrimSpace(script), "echo '") && strings.Contains(script, " > ") {
			parts := strings.SplitN(script, " > ", 2)
			if len(parts) == 2 {
				contentPart := parts[0]
				filePart := strings.TrimSpace(parts[1])

				// Extract content from echo '...'
				content := strings.TrimPrefix(strings.TrimSpace(contentPart), "echo '")
				content = strings.TrimSuffix(content, "'")

				// Unescape newlines
				content = strings.ReplaceAll(content, "\\n", "\n")

				m.filesMu.Lock()
				m.files[filePart] = content
				m.filesMu.Unlock()
				output = "" // No stdout for write
			}
		}

		// Simple Read Simulation: cat file
		if strings.HasPrefix(strings.TrimSpace(script), "cat ") {
			file := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(script), "cat "))
			m.filesMu.Lock()
			content, ok := m.files[file]
			m.filesMu.Unlock()
			if ok {
				output = content
			}
		}
	}

	// Return response with simulated output
	server, client := net.Pipe()
	go func() {
		defer server.Close()
		if output != "" {
			server.Write([]byte(output))
		}
	}()

	return types.HijackedResponse{
		Conn:   client,
		Reader: bufio.NewReader(client),
	}, nil
}

func (m *MockAPI) ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error) {
	if m.ContainerExecInspectFunc != nil {
		return m.ContainerExecInspectFunc(ctx, execID)
	}
	return container.ExecInspect{ExitCode: 0}, nil
}

func (m *MockAPI) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	if m.ContainerStopFunc != nil {
		return m.ContainerStopFunc(ctx, containerID, options)
	}
	return nil
}

func (m *MockAPI) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	if m.ContainerRemoveFunc != nil {
		return m.ContainerRemoveFunc(ctx, containerID, options)
	}
	return nil
}

func (m *MockAPI) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	if m.ContainerListFunc != nil {
		return m.ContainerListFunc(ctx, options)
	}
	return []types.Container{}, nil
}

func (m *MockAPI) ContainerKill(ctx context.Context, containerID, signal string) error {
	if m.ContainerKillFunc != nil {
		return m.ContainerKillFunc(ctx, containerID, signal)
	}
	return nil
}

func (m *MockAPI) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// NewMockClient creates a new Docker client with a configurable mock API.
// It returns the Client wrapper and the underlying MockAPI struct for configuration.
func NewMockClient() (*Client, *MockAPI) {
	mock := &MockAPI{}
	return &Client{api: mock}, mock
}
