package docker

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

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
