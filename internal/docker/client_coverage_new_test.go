package docker

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

func TestExecAsUser_CreateError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		return types.IDResponse{}, errors.New("exec create failed")
	}

	_, err := client.ExecAsUser(context.Background(), "container", "user", []string{"ls"})
	if err == nil {
		t.Error("Expected error from ExecAsUser when create fails")
	}
	if !strings.Contains(err.Error(), "exec create failed") {
		t.Errorf("Expected error to contain 'exec create failed', got: %v", err)
	}
}

func TestExecAsUser_AttachError(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		return types.HijackedResponse{}, errors.New("exec attach failed")
	}

	_, err := client.ExecAsUser(context.Background(), "container", "user", []string{"ls"})
	if err == nil {
		t.Error("Expected error from ExecAsUser when attach fails")
	}
	if !strings.Contains(err.Error(), "exec attach failed") {
		t.Errorf("Expected error to contain 'exec attach failed', got: %v", err)
	}
}

func TestWaitContainer_Error(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerWaitFunc = func(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
		errCh := make(chan error, 1)
		errCh <- errors.New("wait failed")
		return nil, errCh
	}

	_, err := client.WaitContainer(context.Background(), "container")
	if err == nil {
		t.Error("Expected error from WaitContainer")
	}
	if !strings.Contains(err.Error(), "wait failed") {
		t.Errorf("Expected error to contain 'wait failed', got: %v", err)
	}
}

func TestWaitContainer_Success(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerWaitFunc = func(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
		statusCh := make(chan container.WaitResponse, 1)
		statusCh <- container.WaitResponse{StatusCode: 0}
		return statusCh, nil
	}

	code, err := client.WaitContainer(context.Background(), "container")
	if err != nil {
		t.Fatalf("WaitContainer failed: %v", err)
	}
	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
}

func TestContainerLogs_Error(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerLogsFunc = func(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error) {
		return nil, errors.New("logs failed")
	}

	_, err := client.ContainerLogs(context.Background(), "container")
	if err == nil {
		t.Error("Expected error from ContainerLogs")
	}
	if !strings.Contains(err.Error(), "logs failed") {
		t.Errorf("Expected error to contain 'logs failed', got: %v", err)
	}
}
