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

func TestServerVersion(t *testing.T) {
	mock := &mockAPIClient{
		serverVersionFunc: func(ctx context.Context) (types.Version, error) {
			return types.Version{Version: "20.10.0"}, nil
		},
	}
	client := &Client{api: mock}

	v, err := client.ServerVersion(context.Background())
	if err != nil {
		t.Fatalf("ServerVersion failed: %v", err)
	}
	if v.Version != "20.10.0" {
		t.Errorf("Expected version 20.10.0, got %s", v.Version)
	}
}

func TestListContainers(t *testing.T) {
	mock := &mockAPIClient{
		containerListFunc: func(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
			// ListContainers passes options through
			return []types.Container{{ID: "c1"}}, nil
		},
	}
	client := &Client{api: mock}

	containers, err := client.ListContainers(context.Background(), container.ListOptions{All: true})
	if err != nil {
		t.Fatalf("ListContainers failed: %v", err)
	}
	if len(containers) != 1 || containers[0].ID != "c1" {
		t.Errorf("Unexpected containers list: %v", containers)
	}
}

func TestRemoveContainer(t *testing.T) {
	forceCalled := false
	mock := &mockAPIClient{
		containerRemoveFunc: func(ctx context.Context, containerID string, options container.RemoveOptions) error {
			if containerID != "c1" {
				t.Errorf("Expected c1, got %s", containerID)
			}
			forceCalled = options.Force
			return nil
		},
	}

	client := &Client{api: mock}
	if err := client.RemoveContainer(context.Background(), "c1", true); err != nil {
		t.Errorf("RemoveContainer failed: %v", err)
	}

	if !forceCalled {
		t.Error("Expected Force=true")
	}
}

// TestExecInteractive is already defined in client_interactive_test.go

func TestRunContainer_CreateFail(t *testing.T) {
	mock := &mockAPIClient{
		imagePullFunc: func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("")), nil
		},
		containerCreateFunc: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
			return container.CreateResponse{}, errors.New("create failed")
		},
	}
	client := &Client{api: mock}

	_, err := client.RunContainer(context.Background(), "img", "/tmp", nil, nil, "")
	if err == nil {
		t.Error("Expected error on create fail")
	}
	if !strings.Contains(err.Error(), "create failed") {
		t.Errorf("Expected 'create failed', got %v", err)
	}
}

func TestRunContainer_StartFail(t *testing.T) {
	mock := &mockAPIClient{
		imagePullFunc: func(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("")), nil
		},
		containerCreateFunc: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
			return container.CreateResponse{ID: "c1"}, nil
		},
		containerStartFunc: func(ctx context.Context, containerID string, options container.StartOptions) error {
			return errors.New("start failed")
		},
	}
	client := &Client{api: mock}

	_, err := client.RunContainer(context.Background(), "img", "/tmp", nil, nil, "")
	if err == nil {
		t.Error("Expected error on start fail")
	}
	if !strings.Contains(err.Error(), "start failed") {
		t.Errorf("Expected 'start failed', got %v", err)
	}
}
