package docker

import (
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

func TestClient_RunContainer_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("Pull Error", func(t *testing.T) {
		client, mock := NewMockClient()
		mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
			return nil, errors.New("pull failed")
		}

		// RunContainer should not fail if pull fails (it says "Best effort" in comments)
		// But let's check code.
		// RunContainer code:
		// reader, err := c.api.ImagePull(...)
		// if err == nil { ... }
		// It ignores pull error!
		// So we must verify it proceeds to Create.

		mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
			return container.CreateResponse{ID: "id"}, nil
		}

		id, err := client.RunContainer(ctx, "img", "/ws", nil, nil, "user")
		if err != nil {
			t.Errorf("RunContainer failed on pull error: %v", err)
		}
		if id != "id" {
			t.Errorf("Expected id 'id', got '%s'", id)
		}
	})

	t.Run("Create Error", func(t *testing.T) {
		client, mock := NewMockClient()
		mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
			return container.CreateResponse{}, errors.New("create failed")
		}

		_, err := client.RunContainer(ctx, "img", "/ws", nil, nil, "user")
		if err == nil {
			t.Error("RunContainer expected error on create failure, got nil")
		}
		if !strings.Contains(err.Error(), "create failed") {
			t.Errorf("Expected error to contain 'create failed', got: %v", err)
		}
	})

	t.Run("Start Error", func(t *testing.T) {
		client, mock := NewMockClient()
		mock.ContainerStartFunc = func(ctx context.Context, containerID string, options container.StartOptions) error {
			return errors.New("start failed")
		}

		_, err := client.RunContainer(ctx, "img", "/ws", nil, nil, "user")
		if err == nil {
			t.Error("RunContainer expected error on start failure, got nil")
		}
		if !strings.Contains(err.Error(), "start failed") {
			t.Errorf("Expected error to contain 'start failed', got: %v", err)
		}
	})
}

func TestClient_Exec_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("Create Error", func(t *testing.T) {
		client, mock := NewMockClient()
		mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
			return types.IDResponse{}, errors.New("exec create failed")
		}

		_, err := client.Exec(ctx, "cid", []string{"cmd"})
		if err == nil {
			t.Error("Exec expected error on create failure, got nil")
		}
		if !strings.Contains(err.Error(), "exec create failed") {
			t.Errorf("Expected error to contain 'exec create failed', got: %v", err)
		}
	})

	t.Run("Attach Error", func(t *testing.T) {
		client, mock := NewMockClient()
		mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
			return types.HijackedResponse{}, errors.New("attach failed")
		}

		_, err := client.Exec(ctx, "cid", []string{"cmd"})
		if err == nil {
			t.Error("Exec expected error on attach failure, got nil")
		}
		if !strings.Contains(err.Error(), "attach failed") {
			t.Errorf("Expected error to contain 'attach failed', got: %v", err)
		}
	})

	t.Run("Exit Code Error", func(t *testing.T) {
		client, mock := NewMockClient()
		mock.ContainerExecInspectFunc = func(ctx context.Context, execID string) (container.ExecInspect, error) {
			return container.ExecInspect{ExitCode: 123}, nil
		}

		_, err := client.Exec(ctx, "cid", []string{"cmd"})
		if err == nil {
			t.Error("Exec expected error on non-zero exit code, got nil")
		}
		if !strings.Contains(err.Error(), "code 123") {
			t.Errorf("Expected error to contain 'code 123', got: %v", err)
		}
	})

	t.Run("Inspect Error", func(t *testing.T) {
		client, mock := NewMockClient()
		mock.ContainerExecInspectFunc = func(ctx context.Context, execID string) (container.ExecInspect, error) {
			return container.ExecInspect{}, errors.New("inspect failed")
		}

		_, err := client.Exec(ctx, "cid", []string{"cmd"})
		if err == nil {
			t.Error("Exec expected error on inspect failure, got nil")
		}
		if !strings.Contains(err.Error(), "inspect failed") {
			t.Errorf("Expected error to contain 'inspect failed', got: %v", err)
		}
	})
}

func TestClient_PullImage_Errors(t *testing.T) {
	ctx := context.Background()
	client, mock := NewMockClient()

	mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return nil, errors.New("pull fatal")
	}

	err := client.PullImage(ctx, "img")
	if err == nil {
		t.Error("PullImage expected error, got nil")
	}
	if !strings.Contains(err.Error(), "pull fatal") {
		t.Errorf("Expected error to contain 'pull fatal', got: %v", err)
	}

	// Test JSON error decoding?
	// The client reads JSON messages.
	mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(`{"errorDetail":{"message":"pull failed remotely"}}`)), nil
	}

	err = client.PullImage(ctx, "img")
	if err == nil {
		t.Error("PullImage expected error from JSON response, got nil")
	}
	// The Client implementation checks msg.Error which is a *JSONError.
	// {"errorDetail":...} maps to Error field in JSONMessage?
	// Let's check docker pkg/jsonmessage/jsonmessage.go definition or assumption.
	// Client code: if msg.Error != nil { return fmt.Errorf("pull failed: %s", msg.Error.Message) }
	// The JSON structure `{"error": "...", "errorDetail": {"message": "..."}}`

	mock.ImagePullFunc = func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(`{"error":"pull failed remotely","errorDetail":{"message":"pull failed remotely"}}`)), nil
	}
	err = client.PullImage(ctx, "img")
	if err == nil {
		t.Error("PullImage expected error from JSON response, got nil")
	} else if !strings.Contains(err.Error(), "pull failed remotely") {
		t.Errorf("Expected error to contain 'pull failed remotely', got: %v", err)
	}
}
