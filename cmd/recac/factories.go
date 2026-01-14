package main

import (
	"context"
	"recac/internal/docker"
	"recac/internal/git"
	"recac/internal/runner"
)

// DockerStatsClient defines the interface for clients that can fetch Docker container stats.
// This makes the `ps` command testable.
type DockerStatsClient interface {
	GetContainerStats(ctx context.Context, containerID string) (*docker.ContainerStats, error)
}

var (
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManager()
	}

	gitClientFactory = func() IGitClient {
		return git.NewClient()
	}

	dockerClientFactory = func(project string) (DockerStatsClient, error) {
		return docker.NewClient(project)
	}
)
