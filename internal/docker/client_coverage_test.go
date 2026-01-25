package docker

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
)

func TestClient_RunContainer(t *testing.T) {
	client, mock := NewMockClient()

	// Mock ImagePull (success)
	mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(`{"status":"pulling..."}`)), nil
	}

	// Mock ContainerCreate
	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		assert.Equal(t, "alpine:latest", config.Image)
		assert.Equal(t, "/workspace", config.WorkingDir)
		assert.True(t, config.Tty)
		assert.True(t, config.OpenStdin)
		return container.CreateResponse{ID: "test-container-id"}, nil
	}

	// Mock ContainerStart
	mock.ContainerStartFunc = func(ctx context.Context, containerID string, options container.StartOptions) error {
		assert.Equal(t, "test-container-id", containerID)
		return nil
	}

	id, err := client.RunContainer(context.Background(), "alpine:latest", "/tmp/ws", nil, nil, "")
	assert.NoError(t, err)
	assert.Equal(t, "test-container-id", id)
}

func TestClient_RunContainer_PullFail(t *testing.T) {
	client, mock := NewMockClient()

	// Mock ImagePull (fail)
	mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return nil, errors.New("pull failed")
	}

	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		return container.CreateResponse{ID: "test-container-id"}, nil
	}

	id, err := client.RunContainer(context.Background(), "alpine:latest", "/tmp/ws", nil, nil, "")
	assert.NoError(t, err)
	assert.Equal(t, "test-container-id", id)
}

func TestClient_RunContainer_CreateFail(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(`{}`)), nil
	}

	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		return container.CreateResponse{}, errors.New("create failed")
	}

	_, err := client.RunContainer(context.Background(), "alpine:latest", "/tmp/ws", nil, nil, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create failed")
}

func TestClient_Exec_Coverage(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		assert.Equal(t, "test-container-id", container)
		assert.Equal(t, []string{"echo", "hello"}, config.Cmd)
		assert.True(t, config.AttachStdout)
		assert.True(t, config.AttachStderr)
		return types.IDResponse{ID: "exec-id"}, nil
	}

	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		r, w := net.Pipe()
		go func() {
			// Construct StdCopy frame: [1 (stdout), 0, 0, 0, size...]
			header := make([]byte, 8)
			header[0] = 1 // Stdout
			binary.BigEndian.PutUint32(header[4:], 5) // Length 5
			w.Write(header)
			w.Write([]byte("hello"))
			w.Close()
		}()

		return types.HijackedResponse{
			Reader: bufio.NewReader(r),
			Conn:   r,
		}, nil
	}

	mock.ContainerExecInspectFunc = func(ctx context.Context, execID string) (container.ExecInspect, error) {
		return container.ExecInspect{ExitCode: 0}, nil
	}

	output, err := client.Exec(context.Background(), "test-container-id", []string{"echo", "hello"})
	assert.NoError(t, err)
	assert.Contains(t, output, "hello")
}

func TestClient_Exec_Failure(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		return types.IDResponse{ID: "exec-id"}, nil
	}
	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		r, w := net.Pipe()
		w.Close()
		return types.HijackedResponse{Reader: bufio.NewReader(r), Conn: r}, nil
	}
	mock.ContainerExecInspectFunc = func(ctx context.Context, execID string) (container.ExecInspect, error) {
		return container.ExecInspect{ExitCode: 1}, nil // Non-zero exit code
	}

	_, err := client.Exec(context.Background(), "test-container-id", []string{"cmd"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command exited with code 1")
}

func TestClient_ExecAsUser(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		assert.Equal(t, "user1", config.User)
		return types.IDResponse{ID: "exec-id"}, nil
	}

	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		r, w := net.Pipe()
		w.Close()
		return types.HijackedResponse{Reader: bufio.NewReader(r), Conn: r}, nil
	}

	mock.ContainerExecInspectFunc = func(ctx context.Context, execID string) (container.ExecInspect, error) {
		return container.ExecInspect{ExitCode: 0}, nil
	}

	_, err := client.ExecAsUser(context.Background(), "test-container-id", "user1", []string{"cmd"})
	assert.NoError(t, err)
}

func TestClient_CheckImage_ShortID(t *testing.T) {
	client, mock := NewMockClient()

	mock.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) {
		return []image.Summary{
			{
				ID: "sha256:12345678901234567890",
				RepoTags: []string{"none:none"},
			},
		}, nil
	}

	// Test ID matching logic
	// The client implementation checks:
	// if len(img.ID) >= 12 && len(imageRef) >= 12 && imageRef == img.ID[:12]
	// img.ID is "sha256:12345678901234567890".
	// img.ID[:12] is "sha256:12345" (length 12).

	exists, err := client.CheckImage(context.Background(), "sha256:12345")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = client.CheckImage(context.Background(), "sha256:99999")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestClient_RemoveContainer(t *testing.T) {
	client, mock := NewMockClient()

	called := false
	mock.ContainerRemoveFunc = func(ctx context.Context, containerID string, options container.RemoveOptions) error {
		called = true
		assert.Equal(t, "test-id", containerID)
		assert.True(t, options.Force)
		return nil
	}

	err := client.RemoveContainer(context.Background(), "test-id", true)
	assert.NoError(t, err)
	assert.True(t, called)
}
