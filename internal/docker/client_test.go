package docker

import (
	"context"
	"errors"
	"io"
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
	pingFunc func(ctx context.Context) (types.Ping, error)
	// Add other funcs as needed, or let them panic if unused
}

func (m *mockAPIClient) Ping(ctx context.Context) (types.Ping, error) {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	return types.Ping{}, nil
}

func (m *mockAPIClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockAPIClient) ImageBuild(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error) {
	return types.ImageBuildResponse{Body: io.NopCloser(strings.NewReader(""))}, nil
}

func (m *mockAPIClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
	return container.CreateResponse{}, nil
}

func (m *mockAPIClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return nil
}

func (m *mockAPIClient) ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
	return types.IDResponse{}, nil
}

func (m *mockAPIClient) ContainerExecAttach(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
	return types.HijackedResponse{}, nil
}

func (m *mockAPIClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	return nil
}

func (m *mockAPIClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
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

func TestImageBuild_Success(t *testing.T) {
	client, mock := NewMockClient()
	
	// Configure mock to return successful build
	mock.ImageBuildFunc = func(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error) {
		// Verify build options
		if len(options.Tags) == 0 || options.Tags[0] != "testimage:latest" {
			t.Errorf("Expected tag 'testimage:latest', got %v", options.Tags)
		}
		if options.Dockerfile != "Dockerfile" {
			t.Errorf("Expected Dockerfile 'Dockerfile', got %s", options.Dockerfile)
		}
		
		// Return mock build output with image ID
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
	
	// Verify image ID was extracted
	if imageID == "" {
		t.Fatal("Expected image ID, got empty string")
	}
	if !strings.Contains(imageID, "testimageid") && imageID != "testimage:latest" {
		t.Errorf("Expected image ID to contain 'testimageid' or be tag, got %s", imageID)
	}
}

func TestImageBuild_ErrorHandling(t *testing.T) {
	client, mock := NewMockClient()
	
	// Configure mock to return build error
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
		// Verify build args were passed
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
// This test requires:
// - Running inside a container (/.dockerenv exists)
// - Docker socket mounted at /var/run/docker.sock
// - Proper permissions to access Docker socket (user in docker group or root)
// - Ability to create nested containers
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
	// Try to create client - this will fail if we don't have proper permissions
	client, err := NewClient()
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
	// Use a lightweight image like alpine or busybox
	testImage := "alpine:latest"
	testWorkspace := "/tmp/dind-test-workspace"
	
	// Create a temporary workspace directory
	if err := os.MkdirAll(testWorkspace, 0755); err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}
	defer os.RemoveAll(testWorkspace)

	// Create nested container
	containerID, err := client.RunContainer(ctx, testImage, testWorkspace)
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
	// Execute a simple command in the nested container to verify it's running
	output, err := client.Exec(ctx, containerID, []string{"echo", "hello from nested container"})
	if err != nil {
		t.Fatalf("Failed to execute command in nested container: %v", err)
	}

	// Verify output
	expectedOutput := "hello from nested container"
	if !strings.Contains(output, expectedOutput) {
		t.Errorf("Expected output to contain '%s', got: %s", expectedOutput, output)
	}

	// Test passes if we reach here - nested container was created and is functional
	t.Logf("Successfully created and verified nested container %s in DinD environment", containerID)
}
