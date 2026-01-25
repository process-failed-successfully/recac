package docker

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type mockAPIClient struct {
	pingFunc                func(ctx context.Context) (types.Ping, error)
	serverVersionFunc       func(ctx context.Context) (types.Version, error)
	imageListFunc           func(ctx context.Context, options image.ListOptions) ([]image.Summary, error)
	imagePullFunc           func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	containerStopFunc       func(ctx context.Context, containerID string, options container.StopOptions) error
	containerCreateFunc     func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error)
	containerExecCreateFunc func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error)
	containerExecAttachFunc func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error)
	containerListFunc       func(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	containerKillFunc       func(ctx context.Context, containerID, signal string) error
	containerRemoveFunc     func(ctx context.Context, containerID string, options container.RemoveOptions) error
	containerStartFunc      func(ctx context.Context, containerID string, options container.StartOptions) error
}

func (m *mockAPIClient) Ping(ctx context.Context) (types.Ping, error) {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	return types.Ping{}, nil
}

func (m *mockAPIClient) ServerVersion(ctx context.Context) (types.Version, error) {
	if m.serverVersionFunc != nil {
		return m.serverVersionFunc(ctx)
	}
	return types.Version{}, nil
}

func (m *mockAPIClient) ImageList(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
	if m.imageListFunc != nil {
		return m.imageListFunc(ctx, options)
	}
	return []image.Summary{}, nil
}

func (m *mockAPIClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	if m.imagePullFunc != nil {
		return m.imagePullFunc(ctx, ref, options)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockAPIClient) ImageBuild(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error) {
	return types.ImageBuildResponse{Body: io.NopCloser(strings.NewReader(""))}, nil
}

func (m *mockAPIClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
	if m.containerCreateFunc != nil {
		return m.containerCreateFunc(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{ID: "mock-id"}, nil
}

func (m *mockAPIClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	if m.containerStartFunc != nil {
		return m.containerStartFunc(ctx, containerID, options)
	}
	return nil
}

func (m *mockAPIClient) ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
	if m.containerExecCreateFunc != nil {
		return m.containerExecCreateFunc(ctx, container, config)
	}
	return types.IDResponse{}, nil
}

func (m *mockAPIClient) ContainerExecAttach(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
	if m.containerExecAttachFunc != nil {
		return m.containerExecAttachFunc(ctx, execID, config)
	}
	// We need a non-nil Conn to avoid panic in Close()
	return types.HijackedResponse{
		Reader: bufio.NewReader(strings.NewReader("")),
		Conn:   &net.TCPConn{},
	}, nil
}

func (m *mockAPIClient) ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error) {
	return container.ExecInspect{ExitCode: 0}, nil
}

func (m *mockAPIClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	if m.containerStopFunc != nil {
		return m.containerStopFunc(ctx, containerID, options)
	}
	return nil
}

func (m *mockAPIClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	if m.containerRemoveFunc != nil {
		return m.containerRemoveFunc(ctx, containerID, options)
	}
	if containerID == "fail" {
		return errors.New("remove failed")
	}
	return nil
}

func (m *mockAPIClient) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	if m.containerListFunc != nil {
		return m.containerListFunc(ctx, options)
	}
	return []types.Container{}, nil
}

func (m *mockAPIClient) ContainerKill(ctx context.Context, containerID, signal string) error {
	if m.containerKillFunc != nil {
		return m.containerKillFunc(ctx, containerID, signal)
	}
	return nil
}

func (m *mockAPIClient) Close() error {
	return nil
}

func TestCheckDaemon_Success(t *testing.T) {
	mock := &mockAPIClient{
		pingFunc: func(ctx context.Context) (types.Ping, error) {
			return types.Ping{}, nil
		},
	}
	client := &Client{api: mock}

	if err := client.CheckDaemon(context.Background()); err != nil {
		t.Fatalf("CheckDaemon failed: %v", err)
	}
}

func TestCheckDaemon_Failure(t *testing.T) {
	mock := &mockAPIClient{
		pingFunc: func(ctx context.Context) (types.Ping, error) {
			return types.Ping{}, errors.New("daemon down")
		},
	}
	client := &Client{api: mock}

	if err := client.CheckDaemon(context.Background()); err == nil {
		t.Fatal("CheckDaemon expected error, got nil")
	}
}

func TestCheckSocket(t *testing.T) {
	mock := &mockAPIClient{
		pingFunc: func(ctx context.Context) (types.Ping, error) {
			return types.Ping{}, nil
		},
	}
	client := &Client{api: mock}
	if err := client.CheckSocket(context.Background()); err != nil {
		t.Fatalf("CheckSocket failed: %v", err)
	}
}

func TestCheckImage(t *testing.T) {
	mock := &mockAPIClient{
		imageListFunc: func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
			return []image.Summary{
				{RepoTags: []string{"alpine:latest"}},
			}, nil
		},
	}
	client := &Client{api: mock}

	exists, err := client.CheckImage(context.Background(), "alpine:latest")
	if err != nil {
		t.Fatalf("CheckImage failed: %v", err)
	}
	if !exists {
		t.Error("Expected image to exist")
	}

	exists, err = client.CheckImage(context.Background(), "ubuntu:latest")
	if err != nil {
		t.Fatalf("CheckImage failed: %v", err)
	}
	if exists {
		t.Error("Expected image not to exist")
	}
}

func TestPullImage(t *testing.T) {
	mock := &mockAPIClient{
		imagePullFunc: func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"pulling..."}`)), nil
		},
	}
	client := &Client{api: mock}

	if err := client.PullImage(context.Background(), "alpine:latest"); err != nil {
		t.Fatalf("PullImage failed: %v", err)
	}
}

func TestStopContainer(t *testing.T) {
	mock := &mockAPIClient{
		containerStopFunc: func(ctx context.Context, containerID string, options container.StopOptions) error {
			if containerID == "fail" {
				return errors.New("stop failed")
			}
			return nil
		},
	}
	client := &Client{api: mock}

	if err := client.StopContainer(context.Background(), "good"); err != nil {
		t.Errorf("StopContainer failed: %v", err)
	}

	if err := client.StopContainer(context.Background(), "fail"); err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestImageBuild_Success(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImageBuildFunc = func(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error) {
		if len(options.Tags) == 0 || options.Tags[0] != "testimage:latest" {
			t.Errorf("Expected tag 'testimage:latest', got %v", options.Tags)
		}
		if options.Dockerfile != "Dockerfile" {
			t.Errorf("Expected Dockerfile 'Dockerfile', got %s", options.Dockerfile)
		}

		mockOutput := `{"stream":"Step 1/2 : FROM alpine\n"}
{"stream":" ---> abc123def456\n"}
{"aux":{"ID":"sha256:testimageid123456789"}}
{"stream":"Successfully built testimageid123456789\n"}`
		return types.ImageBuildResponse{
			Body: io.NopCloser(strings.NewReader(mockOutput)),
		}, nil
	}

	buildContext := strings.NewReader("mock build context")
	opts := ImageBuildOptions{
		BuildContext: buildContext,
		Tag:          "testimage:latest",
		Dockerfile:   "Dockerfile",
	}

	imageID, err := client.ImageBuild(context.Background(), opts)
	if err != nil {
		t.Fatalf("ImageBuild failed: %v", err)
	}
	if imageID == "" {
		t.Fatal("Expected image ID, got empty string")
	}
}

func TestImageBuild_ErrorHandling(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImageBuildFunc = func(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error) {
		mockOutput := `{"errorDetail":{"message":"Build failed: syntax error"}}`
		return types.ImageBuildResponse{
			Body: io.NopCloser(strings.NewReader(mockOutput)),
		}, nil
	}

	buildContext := strings.NewReader("mock build context")
	opts := ImageBuildOptions{
		BuildContext: buildContext,
		Tag:          "testimage:latest",
	}

	_, err := client.ImageBuild(context.Background(), opts)
	if err == nil {
		t.Fatal("Expected error from ImageBuild, got nil")
	}
	if !strings.Contains(err.Error(), "Build failed") {
		t.Errorf("Expected error to contain 'Build failed', got: %v", err)
	}
}

func TestImageBuild_MissingBuildContext(t *testing.T) {
	client, _ := NewMockClient()

	opts := ImageBuildOptions{
		Tag: "testimage:latest",
		// BuildContext is nil
	}

	_, err := client.ImageBuild(context.Background(), opts)
	if err == nil {
		t.Fatal("Expected error for missing build context, got nil")
	}
	if !strings.Contains(err.Error(), "build context is required") {
		t.Errorf("Expected error about build context, got: %v", err)
	}
}

func TestImageBuild_MissingTag(t *testing.T) {
	client, _ := NewMockClient()

	opts := ImageBuildOptions{
		BuildContext: strings.NewReader("mock context"),
		// Tag is empty
	}

	_, err := client.ImageBuild(context.Background(), opts)
	if err == nil {
		t.Fatal("Expected error for missing tag, got nil")
	}
	if !strings.Contains(err.Error(), "image tag is required") {
		t.Errorf("Expected error about image tag, got: %v", err)
	}
}

func TestImageBuild_WithBuildArgs(t *testing.T) {
	client, mock := NewMockClient()

	version := "1.0.0"
	mock.ImageBuildFunc = func(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error) {
		if options.BuildArgs == nil {
			t.Error("Expected build args, got nil")
		}
		if val, ok := options.BuildArgs["VERSION"]; !ok || val == nil || *val != "1.0.0" {
			t.Errorf("Expected VERSION=1.0.0 in build args, got %v", options.BuildArgs)
		}

		mockOutput := `{"stream":"Successfully built testimageid\n"}`
		return types.ImageBuildResponse{
			Body: io.NopCloser(strings.NewReader(mockOutput)),
		}, nil
	}

	buildContext := strings.NewReader("mock build context")
	opts := ImageBuildOptions{
		BuildContext: buildContext,
		Tag:          "testimage:latest",
		BuildArgs:    map[string]*string{"VERSION": &version},
	}

	_, err := client.ImageBuild(context.Background(), opts)
	if err != nil {
		t.Fatalf("ImageBuild failed: %v", err)
	}
}

// TestDockerInDocker_Support verifies Docker-in-Docker (DinD) support.
func TestDockerInDocker_Support(t *testing.T) {
	// Step 1: Verify we're running inside a container with Docker socket mounted
	if _, err := os.Stat("/.dockerenv"); os.IsNotExist(err) {
		t.Skip("Skipping DinD test: not running in a container (/.dockerenv not found)")
	}

	// Check if Docker socket exists
	if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
		t.Skip("Skipping DinD test: Docker socket not mounted (/var/run/docker.sock not found)")
	}

	// Step 2: Create a real Docker client (not mock) to test actual DinD functionality
	client, err := NewClient("test-dind")
	if err != nil {
		t.Skipf("Skipping DinD test: failed to create Docker client (may need docker group membership): %v", err)
	}
	defer client.Close()

	// Verify Docker daemon is accessible
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.CheckDaemon(ctx); err != nil {
		t.Skipf("Skipping DinD test: Docker daemon not accessible (may need proper permissions): %v", err)
	}

	// Step 3: Create a nested container (worker container)
	testImage := "alpine:latest"
	testWorkspace := "/tmp/dind-test-workspace"

	if err := os.MkdirAll(testWorkspace, 0755); err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}
	defer os.RemoveAll(testWorkspace)

	// Create nested container
	containerID, err := client.RunContainer(ctx, testImage, testWorkspace, nil, nil, "")
	if err != nil {
		t.Fatalf("Failed to create nested container: %v", err)
	}

	// Ensure cleanup
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.StopContainer(stopCtx, containerID); err != nil {
			t.Logf("Warning: Failed to stop container %s: %v", containerID, err)
		}
	}()

	// Step 4: Verify the nested container started correctly
	output, err := client.Exec(ctx, containerID, []string{"echo", "hello from nested container"})
	if err != nil {
		t.Fatalf("Failed to execute command in nested container: %v", err)
	}

	expectedOutput := "hello from nested container"
	if !strings.Contains(output, expectedOutput) {
		t.Errorf("Expected output to contain '%s', got: %s", expectedOutput, output)
	}

	t.Logf("Successfully created and verified nested container %s in DinD environment", containerID)
}

func TestExec_WorkingDir(t *testing.T) {
	var capturedConfig container.ExecOptions
	mock := &mockAPIClient{
		containerExecCreateFunc: func(ctx context.Context, containerID string, config container.ExecOptions) (types.IDResponse, error) {
			capturedConfig = config
			return types.IDResponse{ID: "exec-id"}, nil
		},
	}
	client := &Client{api: mock}

	_, _ = client.Exec(context.Background(), "container-id", []string{"ls"})

	if capturedConfig.WorkingDir != "/workspace" {
		t.Errorf("expected WorkingDir /workspace, got %s", capturedConfig.WorkingDir)
	}
}

func TestExecAsUser_WorkingDir(t *testing.T) {
	var capturedConfig container.ExecOptions
	mock := &mockAPIClient{
		containerExecCreateFunc: func(ctx context.Context, containerID string, config container.ExecOptions) (types.IDResponse, error) {
			capturedConfig = config
			return types.IDResponse{ID: "exec-id"}, nil
		},
	}
	client := &Client{api: mock}

	_, _ = client.ExecAsUser(context.Background(), "container-id", "user", []string{"ls"})

	if capturedConfig.WorkingDir != "/workspace" {
		t.Errorf("expected WorkingDir /workspace, got %s", capturedConfig.WorkingDir)
	}
}
