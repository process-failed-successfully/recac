package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// DockerJanitorClient adapts DockerClient to JanitorClient interface.
type DockerJanitorClient struct {
	client DockerClient
}

// NewDockerJanitorClient creates a new DockerJanitorClient.
func NewDockerJanitorClient(client DockerClient) *DockerJanitorClient {
	return &DockerJanitorClient{
		client: client,
	}
}

// ListCandidates returns Docker containers managed by the orchestrator.
func (d *DockerJanitorClient) ListCandidates(ctx context.Context) ([]Candidate, error) {
	// Create filters to only list containers created by recac-orchestrator
	args := filters.NewArgs()
	args.Add("label", "created-by=recac-orchestrator")

	containers, err := d.client.ListContainers(ctx, container.ListOptions{
		All:     true, // We want stopped containers too
		Filters: args,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var candidates []Candidate
	for _, c := range containers {
		// Map to Candidate
		candidates = append(candidates, Candidate{
			ID:        c.ID,
			Name:      c.ID, // Docker ID is good enough, names might be array
			WorkItem:  c.Labels["work-item"], // Might be empty
			CreatedAt: time.Unix(c.Created, 0),
			Labels:    c.Labels,
		})
	}
	return candidates, nil
}

// Remove deletes the container.
func (d *DockerJanitorClient) Remove(ctx context.Context, id string) error {
	return d.client.RemoveContainer(ctx, id, true)
}
