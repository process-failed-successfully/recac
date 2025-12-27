package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/docker"
)

type Session struct {
	Docker    *docker.Client
	Agent     agent.Agent
	Workspace string
	Image     string
	SpecFile  string
}

func NewSession(d *docker.Client, a agent.Agent, workspace, image string) *Session {
	return &Session{
		Docker:    d,
		Agent:     a,
		Workspace: workspace,
		Image:     image,
		SpecFile:  "app_spec.txt",
	}
}

// ReadSpec reads the application specification file from the workspace.
func (s *Session) ReadSpec() (string, error) {
	path := filepath.Join(s.Workspace, s.SpecFile)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read spec file: %w", err)
	}
	return string(content), nil
}

// Start initializes the session environment (Docker container).
func (s *Session) Start(ctx context.Context) error {
	fmt.Printf("Initializing session with image: %s\n", s.Image)

	// Check Docker Daemon
	if err := s.Docker.CheckDaemon(ctx); err != nil {
		return fmt.Errorf("docker check failed: %w", err)
	}

	// Read Spec
	spec, err := s.ReadSpec()
	if err != nil {
		fmt.Printf("Warning: Failed to read spec: %v\n", err)
	} else {
		fmt.Printf("Loaded spec: %d bytes\n", len(spec))
	}

	// Run Container
	id, err := s.Docker.RunContainer(ctx, s.Image, s.Workspace)
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	fmt.Printf("Container started successfully. ID: %s\n", id)
	return nil
}
