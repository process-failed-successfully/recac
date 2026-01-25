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

func TestImageExists_Error(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
		return nil, errors.New("list failed")
	}

	exists, err := client.ImageExists(context.Background(), "some:tag")
	if err == nil {
		t.Error("Expected error from ImageExists, got nil")
	}
	if exists {
		t.Error("Expected exists=false on error")
	}
}

func TestCheckImage_Extended(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
		return []image.Summary{
			{
				ID:       "sha256:1234567890abcdef1234567890abcdef",
				RepoTags: []string{"myrepo/myimage:v1"},
			},
		}, nil
	}

	tests := []struct {
		name     string
		imageRef string
		want     bool
	}{
		{
			name:     "Exact match tag",
			imageRef: "myrepo/myimage:v1",
			want:     true,
		},
		{
			name:     "Full ID match",
			imageRef: "sha256:1234567890abcdef1234567890abcdef",
			want:     true,
		},
		{
			name:     "Short ID match",
			imageRef: "sha256:12345", // 12 chars
			want:     true,
		},
		{
			name:     "No match",
			imageRef: "other:latest",
			want:     false,
		},
		{
			name:     "Normalization to latest (no match)",
			imageRef: "myrepo/myimage", // -> :latest, which is not v1
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.CheckImage(context.Background(), tt.imageRef)
			if err != nil {
				t.Fatalf("CheckImage failed: %v", err)
			}
			if got != tt.want {
				t.Errorf("CheckImage(%q) = %v, want %v", tt.imageRef, got, tt.want)
			}
		})
	}
}

func TestCheckImage_NormalizationMatch(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
		return []image.Summary{
			{
				RepoTags: []string{"myrepo/myimage:latest"},
			},
		}, nil
	}

	// Should match "myrepo/myimage" because it normalizes to "myrepo/myimage:latest"
	got, err := client.CheckImage(context.Background(), "myrepo/myimage")
	if err != nil {
		t.Fatalf("CheckImage failed: %v", err)
	}
	if !got {
		t.Error("Expected match for normalized latest tag")
	}
}

func TestCheckImage_Error(t *testing.T) {
	client, mock := NewMockClient()
	mock.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
		return nil, errors.New("list failed")
	}

	_, err := client.CheckImage(context.Background(), "foo")
	if err == nil {
		t.Error("Expected error from CheckImage")
	}
}

func TestPullImage_PullError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return nil, errors.New("pull failed")
	}

	err := client.PullImage(context.Background(), "some:image")
	if err == nil {
		t.Error("Expected error from PullImage")
	}
}

func TestRunContainer_PullFailureIgnored(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return nil, errors.New("pull failed")
	}
	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		return container.CreateResponse{ID: "new-id"}, nil
	}

	id, err := client.RunContainer(context.Background(), "img", "/ws", nil, nil, "")
	if err != nil {
		t.Fatalf("RunContainer failed despite pull failure: %v", err)
	}
	if id != "new-id" {
		t.Errorf("Expected container ID 'new-id', got %s", id)
	}
}

func TestExecInteractive_CreateError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		return types.IDResponse{}, errors.New("create failed")
	}

	err := client.ExecInteractive(context.Background(), "container", []string{"ls"})
	if err == nil {
		t.Error("Expected error from ExecInteractive when create fails")
	}
}

func TestExecInteractive_AttachError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		return types.HijackedResponse{}, errors.New("attach failed")
	}

	err := client.ExecInteractive(context.Background(), "container", []string{"ls"})
	if err == nil {
		t.Error("Expected error from ExecInteractive when attach fails")
	}
}

func TestExecInteractive_ExitCodeError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecInspectFunc = func(ctx context.Context, execID string) (container.ExecInspect, error) {
		return container.ExecInspect{ExitCode: 123}, nil
	}

	err := client.ExecInteractive(context.Background(), "container", []string{"ls"})
	if err == nil {
		t.Error("Expected error from ExecInteractive when exit code is non-zero")
	}
	if !strings.Contains(err.Error(), "123") {
		t.Errorf("Expected error message to contain '123', got %v", err)
	}
}

func TestListContainers_Error(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerListFunc = func(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
		return nil, errors.New("list failed")
	}

	_, err := client.ListContainers(context.Background(), container.ListOptions{})
	if err == nil {
		t.Error("Expected error from ListContainers")
	}
}

func TestKillContainer_Error(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerKillFunc = func(ctx context.Context, containerID, signal string) error {
		return errors.New("kill failed")
	}

	err := client.KillContainer(context.Background(), "id", "SIGKILL")
	if err == nil {
		t.Error("Expected error from KillContainer")
	}
}

func TestImageBuild_APIError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImageBuildFunc = func(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error) {
		return types.ImageBuildResponse{}, errors.New("build API failed")
	}

	opts := ImageBuildOptions{
		BuildContext: strings.NewReader("ctx"),
		Tag:          "tag",
	}
	_, err := client.ImageBuild(context.Background(), opts)
	if err == nil {
		t.Error("Expected error from ImageBuild API failure")
	}
}

func TestImageBuild_JSONError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImageBuildFunc = func(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error) {
		return types.ImageBuildResponse{
			Body: io.NopCloser(strings.NewReader("{invalid json")),
		}, nil
	}

	opts := ImageBuildOptions{
		BuildContext: strings.NewReader("ctx"),
		Tag:          "tag",
	}
	_, err := client.ImageBuild(context.Background(), opts)
	if err == nil {
		t.Error("Expected error from ImageBuild JSON failure")
	}
}

func TestImageBuild_IDFallback(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImageBuildFunc = func(ctx context.Context, buildContext io.Reader, options build.ImageBuildOptions) (types.ImageBuildResponse, error) {
		// Valid JSON but no ID info
		return types.ImageBuildResponse{
			Body: io.NopCloser(strings.NewReader("{\"stream\": \"Step 1/1 : FROM alpine\"}")),
		}, nil
	}

	opts := ImageBuildOptions{
		BuildContext: strings.NewReader("ctx"),
		Tag:          "my-fallback-tag",
	}
	id, err := client.ImageBuild(context.Background(), opts)
	if err != nil {
		t.Fatalf("ImageBuild failed: %v", err)
	}
	if id != "my-fallback-tag" {
		t.Errorf("Expected fallback to tag 'my-fallback-tag', got %s", id)
	}
}
