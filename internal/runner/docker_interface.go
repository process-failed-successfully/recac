package runner

import (
	"recac/internal/docker"
)

// DockerClient interface abstracts the docker client methods used by Session.
// This allows mocking for unit tests.
type DockerClient interface {
	docker.IClient
}
