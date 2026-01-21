package docker

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestPullImage_DecodeError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		// Return invalid JSON
		return io.NopCloser(strings.NewReader(`{invalid json`)), nil
	}

	err := client.PullImage(context.Background(), "test-image")
	if err == nil {
		t.Error("Expected error from PullImage with invalid JSON")
	}
}

func TestPullImage_ErrorMessage(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(`{"errorDetail":{"message":"pull failed"}}`)), nil
	}

	err := client.PullImage(context.Background(), "test-image")
	if err == nil {
		t.Error("Expected error from PullImage with error message")
	}
	if !strings.Contains(err.Error(), "pull failed") {
		t.Errorf("Expected error containing 'pull failed', got %v", err)
	}
}

func TestRunContainer_CreateError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		return container.CreateResponse{}, errors.New("create failed")
	}

	_, err := client.RunContainer(context.Background(), "image", "/tmp", nil, nil, "")
	if err == nil {
		t.Error("Expected error from RunContainer when create fails")
	}
}

func TestRunContainer_StartError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerStartFunc = func(ctx context.Context, containerID string, options container.StartOptions) error {
		return errors.New("start failed")
	}

	_, err := client.RunContainer(context.Background(), "image", "/tmp", nil, nil, "")
	if err == nil {
		t.Error("Expected error from RunContainer when start fails")
	}
}

func TestExec_InspectError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecInspectFunc = func(ctx context.Context, execID string) (container.ExecInspect, error) {
		return container.ExecInspect{}, errors.New("inspect failed")
	}

	_, err := client.Exec(context.Background(), "container", []string{"ls"})
	if err == nil {
		t.Error("Expected error from Exec when inspect fails")
	}
}

func TestExec_ExitCodeError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecInspectFunc = func(ctx context.Context, execID string) (container.ExecInspect, error) {
		return container.ExecInspect{ExitCode: 1}, nil
	}

	_, err := client.Exec(context.Background(), "container", []string{"ls"})
	if err == nil {
		t.Error("Expected error from Exec when exit code is 1")
	}
}

func TestExecInteractive_InspectError(t *testing.T) {
    client, mock := NewMockClient()

    // Setup NopConn for Attach
    mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		return types.HijackedResponse{
			Conn:   NopConn{},
			Reader: bufio.NewReader(strings.NewReader("")),
		}, nil
	}

    mock.ContainerExecInspectFunc = func(ctx context.Context, execID string) (container.ExecInspect, error) {
        return container.ExecInspect{}, errors.New("inspect failed")
    }

    err := client.ExecInteractive(context.Background(), "container", []string{"bash"})
    if err == nil {
        t.Error("Expected error from ExecInteractive when inspect fails")
    }
}

func TestRemoveContainer_Error(t *testing.T) {
    client, mock := NewMockClient()

    mock.ContainerRemoveFunc = func(ctx context.Context, containerID string, options container.RemoveOptions) error {
        return errors.New("remove failed")
    }

    err := client.RemoveContainer(context.Background(), "container", true)
    if err == nil {
        t.Error("Expected error from RemoveContainer")
    }
}
