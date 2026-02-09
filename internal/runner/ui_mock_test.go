package runner

import (
	"context"
	"recac/internal/db"
	"recac/internal/docker"
)

// UIMockDocker mocks DockerClient for UI tests, capturing signals in a real DB store
type UIMockDocker struct {
	Store db.Store
}

func (m *UIMockDocker) CheckDaemon(ctx context.Context) error { return nil }
func (m *UIMockDocker) ImageExists(ctx context.Context, image string) (bool, error) {
	return true, nil
}
func (m *UIMockDocker) PullImage(ctx context.Context, image string) error { return nil }
func (m *UIMockDocker) ImageBuild(ctx context.Context, opts docker.ImageBuildOptions) (string, error) {
	return "mock-id", nil
}
func (m *UIMockDocker) RunContainer(ctx context.Context, image, workspace string, binds, env []string, user string) (string, error) {
	return "mock-container-id", nil
}
func (m *UIMockDocker) StopContainer(ctx context.Context, id string) error { return nil }
func (m *UIMockDocker) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	// Simulate agent-bridge signal execution
	if len(cmd) > 0 && cmd[0] == "agent-bridge" {
		// Parse simplified command: agent-bridge signal KEY VALUE
		if len(cmd) >= 4 && cmd[1] == "signal" {
			key := cmd[2]
			value := cmd[3]
			// Persist to the attached store
			if m.Store != nil {
				// Use empty string as projectID or match what Session uses
				// Since we don't have projectID passed here, we might need to hardcode or infer
				// But test session sets project to "unknown" by default or whatever logic
				// Actually SetSignal takes (projectID, key, value)
				// The test might not have set a project ID on the session initially, or it defaults.
				// Let's assume default or try to match.
				_ = m.Store.SetSignal("unknown", key, value)
			}
		}
	}
	return "", nil
}

func (m *UIMockDocker) ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error) {
	return m.Exec(ctx, containerID, cmd)
}
