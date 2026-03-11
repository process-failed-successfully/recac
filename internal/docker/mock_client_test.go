package docker

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"net"
	"bufio"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
)

func TestMockClient_Coverage(t *testing.T) {
	_, mock := NewMockClient()
	ctx := context.Background()

	t.Run("Ping", func(t *testing.T) {
		ping, err := mock.Ping(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, ping)
	})

	t.Run("ServerVersion", func(t *testing.T) {
		version, err := mock.ServerVersion(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "1.41", version.APIVersion)
	})

	t.Run("ImageBuild", func(t *testing.T) {
		resp, err := mock.ImageBuild(ctx, strings.NewReader(""), build.ImageBuildOptions{})
		assert.NoError(t, err)
		b, _ := io.ReadAll(resp.Body)
		assert.Contains(t, string(b), "Successfully built")
	})

	t.Run("ContainerStop", func(t *testing.T) {
		err := mock.ContainerStop(ctx, "id", container.StopOptions{})
		assert.NoError(t, err)
	})

	t.Run("Close", func(t *testing.T) {
		err := mock.Close()
		assert.NoError(t, err)
	})

	t.Run("ContainerWait", func(t *testing.T) {
		statusCh, errCh := mock.ContainerWait(ctx, "id", container.WaitConditionNotRunning)
		status := <-statusCh
		assert.Equal(t, int64(0), status.StatusCode)
		assert.Empty(t, errCh)
	})

	t.Run("ImageList", func(t *testing.T) {
		images, err := mock.ImageList(ctx, image.ListOptions{})
		assert.NoError(t, err)
		assert.NotEmpty(t, images)
	})

	t.Run("ImagePull", func(t *testing.T) {
		r, err := mock.ImagePull(ctx, "ref", image.PullOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, r)
	})

	t.Run("ContainerRemove", func(t *testing.T) {
		err := mock.ContainerRemove(ctx, "id", container.RemoveOptions{})
		assert.NoError(t, err)
	})

	t.Run("ContainerList", func(t *testing.T) {
		l, err := mock.ContainerList(ctx, container.ListOptions{})
		assert.NoError(t, err)
		assert.Empty(t, l)
	})

	t.Run("ContainerKill", func(t *testing.T) {
		err := mock.ContainerKill(ctx, "id", "SIGKILL")
		assert.NoError(t, err)
	})

	t.Run("ContainerLogs", func(t *testing.T) {
		r, err := mock.ContainerLogs(ctx, "id", container.LogsOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, r)
	})
}

func TestMockClient_Coverage_Extra2(t *testing.T) {
	_, mock := NewMockClient()
	ctx := context.Background()

	t.Run("Ping_Override", func(t *testing.T) {
		mock.PingFunc = func(ctx context.Context) (types.Ping, error) { return types.Ping{}, io.EOF }
		_, err := mock.Ping(ctx)
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("ServerVersion_Override", func(t *testing.T) {
		mock.ServerVersionFunc = func(ctx context.Context) (types.Version, error) { return types.Version{}, io.EOF }
		_, err := mock.ServerVersion(ctx)
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("ContainerStop_Override", func(t *testing.T) {
		mock.ContainerStopFunc = func(ctx context.Context, id string, opts container.StopOptions) error { return io.EOF }
		err := mock.ContainerStop(ctx, "id", container.StopOptions{})
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("Close_Override", func(t *testing.T) {
		mock.CloseFunc = func() error { return io.EOF }
		err := mock.Close()
		assert.ErrorIs(t, err, io.EOF)
	})
}

func TestClient_ServerVersion(t *testing.T) {
	client, mock := NewMockClient()

	ctx := context.Background()

	version, err := client.ServerVersion(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "1.41", version.APIVersion)

	mock.ServerVersionFunc = func(ctx context.Context) (types.Version, error) { return types.Version{}, io.EOF }
	_, err = client.ServerVersion(ctx)
	assert.ErrorIs(t, err, io.EOF)
}

func TestClient_ImageExists(t *testing.T) {
	client, mock := NewMockClient()

	ctx := context.Background()

	exists, err := client.ImageExists(ctx, "ubuntu:latest")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = client.ImageExists(ctx, "not-found:latest")
	assert.NoError(t, err)
	assert.False(t, exists)

	mock.ImageListFunc = func(ctx context.Context, options image.ListOptions) ([]image.Summary, error) { return nil, io.EOF }
	_, err = client.ImageExists(ctx, "ubuntu:latest")
	assert.ErrorIs(t, err, io.EOF)
}

func TestClient_CheckSocket(t *testing.T) {
	client, mock := NewMockClient()

	ctx := context.Background()

	err := client.CheckSocket(ctx)
	assert.NoError(t, err)

	mock.PingFunc = func(ctx context.Context) (types.Ping, error) { return types.Ping{}, io.EOF }
	err = client.CheckSocket(ctx)
	assert.ErrorIs(t, err, io.EOF)
}

func TestClient_WaitContainer(t *testing.T) {
	client, mock := NewMockClient()
	ctx := context.Background()

	code, err := client.WaitContainer(ctx, "id")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), code)

	mock.ContainerWaitFunc = func(ctx context.Context, id string, cond container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
		errCh := make(chan error, 1)
		errCh <- io.EOF
		return nil, errCh
	}
	_, err = client.WaitContainer(ctx, "id")
	assert.Equal(t, io.EOF, err)
}

func TestClient_WaitContainer_ContextDone(t *testing.T) {
	client, mock := NewMockClient()
	ctx, cancel := context.WithCancel(context.Background())

	mock.ContainerWaitFunc = func(ctx context.Context, id string, cond container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
		statusCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		return statusCh, errCh
	}

	cancel() // cancel immediately
	_, err := client.WaitContainer(ctx, "id")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestClient_RunContainerWithLabels(t *testing.T) {
	client, mock := NewMockClient()
	ctx := context.Background()

	id, err := client.RunContainerWithLabels(ctx, "img", "/workspace", nil, nil, nil, "user", map[string]string{"foo": "bar"})
	assert.NoError(t, err)
	assert.Equal(t, "mock-container-id", id)

	mock.ContainerStartFunc = func(ctx context.Context, id string, opts container.StartOptions) error { return io.EOF }
	_, err = client.RunContainerWithLabels(ctx, "img", "/workspace", nil, nil, nil, "user", nil)
	assert.ErrorIs(t, err, io.EOF)

	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		return container.CreateResponse{}, io.EOF
	}
	_, err = client.RunContainerWithLabels(ctx, "img", "/workspace", nil, nil, nil, "user", nil)
	assert.ErrorIs(t, err, io.EOF)
}

func TestClient_NewClient(t *testing.T) {
	_, err := NewClient("proj")
	// If dockerd isn't running it might return err, but it might just return the client object since NewClientWithOpts doesn't ping.
	// We just want to exercise the code path.
	if err == nil {
		t.Log("NewClient succeeded")
	} else {
		t.Log("NewClient failed", err)
	}
}

func TestClient_Exec_Error(t *testing.T) {
	client, mock := NewMockClient()
	ctx := context.Background()

	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		return types.IDResponse{}, io.EOF
	}
	_, err := client.Exec(ctx, "id", nil)
	assert.ErrorIs(t, err, io.EOF)

	mock.ContainerExecCreateFunc = nil
	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		return types.HijackedResponse{}, io.EOF
	}
	_, err = client.Exec(ctx, "id", nil)
	assert.ErrorIs(t, err, io.EOF)
}

func TestClient_ExecAsUser_Error(t *testing.T) {
	client, mock := NewMockClient()
	ctx := context.Background()

	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		return types.IDResponse{}, io.EOF
	}
	_, err := client.ExecAsUser(ctx, "id", "user", nil)
	assert.ErrorIs(t, err, io.EOF)

	mock.ContainerExecCreateFunc = nil
	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		return types.HijackedResponse{}, io.EOF
	}
	_, err = client.ExecAsUser(ctx, "id", "user", nil)
	assert.ErrorIs(t, err, io.EOF)
}

func TestClient_Exec_InspectError(t *testing.T) {
	client, mock := NewMockClient()
	ctx := context.Background()

	mock.ContainerExecInspectFunc = func(ctx context.Context, execID string) (container.ExecInspect, error) {
		return container.ExecInspect{}, io.EOF
	}

	_, err := client.Exec(ctx, "id", nil)
	assert.ErrorIs(t, err, io.EOF)
}

func TestClient_ExecAsUser_InspectError(t *testing.T) {
	client, mock := NewMockClient()
	ctx := context.Background()

	mock.ContainerExecInspectFunc = func(ctx context.Context, execID string) (container.ExecInspect, error) {
		return container.ExecInspect{}, io.EOF
	}

	_, err := client.ExecAsUser(ctx, "id", "user", nil)
	assert.ErrorIs(t, err, io.EOF)
}

func TestClient_ExecAsUser_ContextCancel(t *testing.T) {
	client, mock := NewMockClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Mock attach to return a real connection but we are immediately canceled
	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		// return a blocked connection
		clientConn, _ := net.Pipe()
		return types.HijackedResponse{
			Conn:   clientConn,
			Reader: bufio.NewReader(clientConn),
		}, nil
	}

	_, err := client.ExecAsUser(ctx, "id", "user", nil)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestClient_Exec_ContextCancel(t *testing.T) {
	client, mock := NewMockClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		clientConn, _ := net.Pipe()
		return types.HijackedResponse{
			Conn:   clientConn,
			Reader: bufio.NewReader(clientConn),
		}, nil
	}

	_, err := client.Exec(ctx, "id", nil)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestClient_RunContainerWithLabels_HostWorkspacePath(t *testing.T) {
	client, _ := NewMockClient()
	client.HostWorkspacePath = "/host/path"
	ctx := context.Background()

	id, err := client.RunContainerWithLabels(ctx, "img", "/workspace", []string{"/extra:/extra"}, nil, []string{"cmd"}, "user", nil)
	assert.NoError(t, err)
	assert.Equal(t, "mock-container-id", id)
}

func TestClient_NewClient_EmptyProject(t *testing.T) {
	client, err := NewClient("")
	// May fail if daemon is down, but we just want coverage
	if err == nil {
		assert.Equal(t, "unknown", client.project)
	}
}

func TestClient_ExecAsUser_CopyError(t *testing.T) {
	client, mock := NewMockClient()
	ctx := context.Background()

	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		// return a connection that fails on read
		clientConn, _ := net.Pipe()
		clientConn.Close() // immediately closed
		return types.HijackedResponse{
			Conn:   clientConn,
			Reader: bufio.NewReader(clientConn),
		}, nil
	}

	_, err := client.ExecAsUser(ctx, "id", "user", nil)
	assert.Error(t, err)
}

func TestClient_Exec_CopyError(t *testing.T) {
	client, mock := NewMockClient()
	ctx := context.Background()

	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		// return a connection that fails on read
		clientConn, _ := net.Pipe()
		clientConn.Close() // immediately closed
		return types.HijackedResponse{
			Conn:   clientConn,
			Reader: bufio.NewReader(clientConn),
		}, nil
	}

	_, err := client.Exec(ctx, "id", nil)
	assert.Error(t, err)
}

func TestClient_ExecAsUser_NonZeroExit(t *testing.T) {
	client, mock := NewMockClient()
	ctx := context.Background()

	mock.ContainerExecInspectFunc = func(ctx context.Context, execID string) (container.ExecInspect, error) {
		return container.ExecInspect{ExitCode: 1}, nil
	}

	_, err := client.ExecAsUser(ctx, "id", "user", nil)
	assert.Error(t, err)
}
