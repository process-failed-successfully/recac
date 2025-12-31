package docker

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestRunContainer_MountsWorkspace(t *testing.T) {
	client, mock := NewMockClient()

	workspacePath := "/tmp/workspace"
	expectedBind := fmt.Sprintf("%s:/workspace", workspacePath)
	bindFound := false

	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		for _, bind := range hostConfig.Binds {
			if bind == expectedBind {
				bindFound = true
			}
		}
		return container.CreateResponse{ID: "test-container"}, nil
	}

	_, err := client.RunContainer(context.Background(), "alpine", workspacePath, nil, "")
	if err != nil {
		t.Fatalf("RunContainer failed: %v", err)
	}

	if !bindFound {
		t.Errorf("Expected bind mount %s not found", expectedBind)
	}
}

func TestRunContainer_SetsWorkingDir(t *testing.T) {
	client, mock := NewMockClient()

	workingDirCorrect := false

	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		if config.WorkingDir == "/workspace" {
			workingDirCorrect = true
		}
		return container.CreateResponse{ID: "test-container"}, nil
	}

	_, err := client.RunContainer(context.Background(), "alpine", "/tmp/ws", nil, "")
	if err != nil {
		t.Fatalf("RunContainer failed: %v", err)
	}

	if !workingDirCorrect {
		t.Error("Expected WorkingDir to be /workspace")
	}
}
