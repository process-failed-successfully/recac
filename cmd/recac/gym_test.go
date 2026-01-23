package main

import (
	"context"
	"testing"

	"recac/internal/agent"
	"recac/internal/docker"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
)

// GymTestMockDockerClient implements runner.DockerClient
type GymTestMockDockerClient struct {
	execOutput string
	execErr    error
}

func (m *GymTestMockDockerClient) CheckDaemon(ctx context.Context) error {
	return nil
}

func (m *GymTestMockDockerClient) RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error) {
	return "mock-container-id", nil
}

func (m *GymTestMockDockerClient) StopContainer(ctx context.Context, containerID string) error {
	return nil
}

func (m *GymTestMockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	return m.execOutput, m.execErr
}

func (m *GymTestMockDockerClient) ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error) {
	return m.execOutput, m.execErr
}

func (m *GymTestMockDockerClient) ImageExists(ctx context.Context, tag string) (bool, error) {
	return true, nil
}

func (m *GymTestMockDockerClient) ImageBuild(ctx context.Context, opts docker.ImageBuildOptions) (string, error) {
	return "mock-image-id", nil
}

func (m *GymTestMockDockerClient) PullImage(ctx context.Context, imageRef string) error {
	return nil
}

func (m *GymTestMockDockerClient) Close() error {
	return nil
}

// GymTestMockAgent implements agent.Agent
type GymTestMockAgent struct {
	response string
}

func (m *GymTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.response, nil
}

func (m *GymTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.response, nil
}

func TestRunGymSession(t *testing.T) {
	// Setup Mocks
	mockDocker := &GymTestMockDockerClient{
		execOutput: "PASSED",
	}
	mockAgent := &GymTestMockAgent{
		response: "I have completed the task.\nCOMPLETED",
	}

	// Override Factories
	originalDockerFactory := gymDockerClientFactory
	originalAgentFactory := gymAgentFactory
	originalSessionFactory := gymSessionFactory
	defer func() {
		gymDockerClientFactory = originalDockerFactory
		gymAgentFactory = originalAgentFactory
		gymSessionFactory = originalSessionFactory
	}()

	gymDockerClientFactory = func(project string) (runner.DockerClient, error) {
		return mockDocker, nil
	}

	gymAgentFactory = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Override NewSession to avoid real Session logic if needed,
	// but we want to test the flow.
	// Since we mock Docker and Agent, Session should be mostly side-effect free (DB is local/sqlite).
	// However, Session does DB init which might fail or be slow.
	// And it does `os.Mkdir` for logs.
	// For now, let's try with real Session but mocked components.

	challenge := GymChallenge{
		Name:        "Test Challenge",
		Description: "Do something",
		Language:    "python",
		TestFile:    "test.py",
		Tests:       "print('PASSED')",
		Timeout:     5,
	}

	// Run
	result, err := runGymSession(context.Background(), challenge)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Passed)
	assert.Equal(t, "Test Challenge", result.Challenge)
}
