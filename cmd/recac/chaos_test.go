package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/docker"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
)

func TestChaosDocker(t *testing.T) {
	// Setup Mock
	client, mockAPI := docker.NewMockClient()

	killed := false
	mockAPI.ContainerListFunc = func(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
		// Verify filters
		if !options.Filters.ExactMatch("name", "test-agent") {
			return nil, fmt.Errorf("unexpected filter")
		}
		return []types.Container{
			{
				ID:    "container123",
				Names: []string{"/test-agent-1"},
			},
		}, nil
	}
	mockAPI.ContainerRemoveFunc = func(ctx context.Context, containerID string, options container.RemoveOptions) error {
		if containerID == "container123" && options.Force {
			killed = true
		}
		return nil
	}

	// Override factory
	origFactory := chaosDockerClientFactory
	chaosDockerClientFactory = func() (*docker.Client, error) {
		return client, nil
	}
	defer func() { chaosDockerClientFactory = origFactory }()

	// Execute
	cmd := chaosDockerCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Set flags
	chaosTarget = "test-agent"
	chaosDuration = 100 * time.Millisecond
	chaosInterval = 50 * time.Millisecond

	err := runChaosDocker(cmd, []string{})
	assert.NoError(t, err)
	assert.True(t, killed, "Expected container to be killed")
	assert.Contains(t, buf.String(), "💀 Killing container")
}

func TestChaosFile(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(targetFile, []byte("hello"), 0644)
	assert.NoError(t, err)

	infoBefore, err := os.Stat(targetFile)
	assert.NoError(t, err)

	cmd := chaosFileCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	chaosPath = tmpDir
	chaosDuration = 100 * time.Millisecond
	chaosInterval = 50 * time.Millisecond

	err = runChaosFile(cmd, []string{})
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "✍️ Touching file")

	infoAfter, err := os.Stat(targetFile)
	assert.NoError(t, err)

	// ModTime check might be flaky depending on FS resolution, so we rely on output verification mostly
	// But valid check:
	assert.True(t, !infoAfter.ModTime().Before(infoBefore.ModTime()))
}

func TestChaosStress(t *testing.T) {
	cmd := chaosStressCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	chaosCPU = 1
	chaosDuration = 50 * time.Millisecond

	err := runChaosStress(cmd, []string{})
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "⚡ Starting CPU Stress")
	assert.Contains(t, buf.String(), "🛑 Stress finished")
}
