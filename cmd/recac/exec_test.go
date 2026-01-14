package main

import (
	"context"
	"fmt"
	"recac/internal/docker"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExecCommand(t *testing.T) {
	// --- Setup ---
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	// Use a real docker client to start a container
	dockerClient, err := docker.NewClient("recac-test")
	require.NoError(t, err)

	// Use a simple image that's likely to be available
	imageRef := "alpine:latest"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Ensure the image is present
	exists, err := dockerClient.ImageExists(ctx, imageRef)
	require.NoError(t, err)
	if !exists {
		err := dockerClient.PullImage(ctx, imageRef)
		require.NoError(t, err, "Failed to pull alpine image for test")
	}

	// Start a long-running container
	containerID, err := dockerClient.RunContainer(ctx, imageRef, t.TempDir(), nil, nil, "")
	require.NoError(t, err)
	defer func() {
		// Stop and remove the container to clean up
		_ = dockerClient.StopContainer(context.Background(), containerID)
		_ = dockerClient.RemoveContainer(context.Background(), containerID, true)
	}()

	// Create a mock session associated with the real container
	sessionName := "exec-test-session"
	mockSession := &runner.SessionState{
		Name:        sessionName,
		Status:      "running",
		StartTime:   time.Now(),
		ContainerID: containerID,
	}
	require.NoError(t, sm.SaveSession(mockSession))

	// --- Execution ---
	// Give the container a moment to be fully up
	time.Sleep(1 * time.Second)

	// Execute a command in the container via the `exec` command
	cmdToExec := "echo 'hello from container'"
	output, err := executeCommand(rootCmd, "exec", sessionName, "/bin/sh", "-c", cmdToExec)
	require.NoError(t, err)

	// --- Assertions ---
	// The output should be exactly what the echo command produced
	expectedOutput := "hello from container"
	require.Equal(t, expectedOutput, strings.TrimSpace(output))
}

func TestExecCommand_NoContainer(t *testing.T) {
	// --- Setup ---
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	sessionName := "no-container-session"
	mockSession := &runner.SessionState{
		Name:   sessionName,
		Status: "running", // A running session that somehow has no container
	}
	require.NoError(t, sm.SaveSession(mockSession))

	// --- Execution ---
	_, err := executeCommand(rootCmd, "exec", sessionName, "ls")

	// --- Assertions ---
	require.Error(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("session '%s' is not associated with a container", sessionName))
}

func TestExecCommand_SessionNotFound(t *testing.T) {
	// --- Setup ---
	_, cleanup := setupTestSessionManager(t)
	defer cleanup()

	// --- Execution ---
	_, err := executeCommand(rootCmd, "exec", "non-existent-session", "ls")

	// --- Assertions ---
	require.Error(t, err)
	require.Contains(t, err.Error(), "session not found")
}
