package main

import (
	"bytes"
	"context"
	"errors"
	"recac/internal/docker"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestShowStatus(t *testing.T) {
	_, sm, cleanup := setupTestEnvironment(t)
	defer cleanup()

	sessionName := "test-session-running"
	createFakeSession(t, sm, sessionName, "running")

	viper.Set("provider", "test-provider")
	viper.Set("model", "test-model")
	viper.Set("config", "/tmp/config.yaml")
	defer viper.Reset()

	t.Run("Success with Running Container", func(t *testing.T) {
		mockClient, mockAPI := docker.NewMockClient()
		mockAPI.ContainerInspectFunc = func(ctx context.Context, containerID string) (types.ContainerJSON, error) {
			return types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					State: &types.ContainerState{Status: "running"},
				},
			}, nil
		}

		var out bytes.Buffer
		err := showStatus(&out, sm, mockClient, nil)
		assert.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "NAME")
		assert.Contains(t, output, "PID")
		assert.Contains(t, output, "SESSION STATUS")
		assert.Contains(t, output, "CONTAINER")
		assert.Contains(t, output, sessionName)
		assert.Contains(t, output, "RUNNING")
		assert.Contains(t, output, "[Logs]")
		assert.Contains(t, output, "recac logs "+sessionName)
	})

	t.Run("Container Not Found", func(t *testing.T) {
		mockClient, mockAPI := docker.NewMockClient()
		mockAPI.ContainerInspectFunc = func(ctx context.Context, containerID string) (types.ContainerJSON, error) {
			return types.ContainerJSON{}, errors.New("container not found")
		}

		var out bytes.Buffer
		err := showStatus(&out, sm, mockClient, nil)
		assert.NoError(t, err)
		assert.Contains(t, out.String(), "NOT FOUND")
	})

	t.Run("No Sessions", func(t *testing.T) {
		// Create a new, empty session manager for this test
		_, emptySm, emptyCleanup := setupTestEnvironment(t)
		defer emptyCleanup()
		mockClient, _ := docker.NewMockClient()

		var out bytes.Buffer
		err := showStatus(&out, emptySm, mockClient, nil)
		assert.NoError(t, err)
		assert.Contains(t, out.String(), "No active or past sessions found.")
	})

	t.Run("Docker Client Initialization Error", func(t *testing.T) {
		dockerErr := errors.New("docker daemon not available")

		var out bytes.Buffer
		err := showStatus(&out, sm, nil, dockerErr)
		assert.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "Docker client failed to initialize")
		assert.Contains(t, output, dockerErr.Error())
	})
}
