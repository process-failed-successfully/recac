package docker

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestRunContainer_EnvVars(t *testing.T) {
	// Setup Mock
	var capturedConfig *container.Config
	mock := &mockAPIClient{
		containerCreateFunc: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
			capturedConfig = config
			return container.CreateResponse{ID: "mock-id"}, nil
		},
	}
	client := &Client{api: mock}

	// Execute RunContainer with intended Env Vars (passed as "ports" currently)
	expectedEnv := []string{"KEY=VALUE", "RECAC_PROJECT_ID=test-project"}
	_, err := client.RunContainer(context.Background(), "image", "/tmp", nil, expectedEnv, "")
	if err != nil {
		t.Fatalf("RunContainer failed: %v", err)
	}

	// Verify
	if capturedConfig == nil {
		t.Fatal("ContainerCreate was not called")
	}

	foundKey := false
	foundProject := false
	for _, e := range capturedConfig.Env {
		if e == "KEY=VALUE" {
			foundKey = true
		}
		if e == "RECAC_PROJECT_ID=test-project" {
			foundProject = true
		}
	}

	if !foundKey {
		t.Errorf("Expected Env 'KEY=VALUE' to be present in container config, but it was missing. Env: %v", capturedConfig.Env)
	}
	if !foundProject {
		t.Errorf("Expected Env 'RECAC_PROJECT_ID=test-project' to be present in container config, but it was missing. Env: %v", capturedConfig.Env)
	}
}
