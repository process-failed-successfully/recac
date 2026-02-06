package main

import (
	"context"
	"os"
	"path/filepath"
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

func TestLoadChallenges_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a nested directory structure
	nestedDir := filepath.Join(tmpDir, "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}

	// Create yaml challenge in root
	yamlContent := `
- name: Challenge 1
  description: Desc 1
  language: python
  test_file: test1.py
  tests: print("1")
`
	if err := os.WriteFile(filepath.Join(tmpDir, "c1.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write yaml: %v", err)
	}

	// Create json challenge in nested
	jsonContent := `[
  {
    "name": "Challenge 2",
    "description": "Desc 2",
    "language": "python",
    "test_file": "test2.py",
    "tests": "print(\"2\")"
  }
]`
	if err := os.WriteFile(filepath.Join(nestedDir, "c2.json"), []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to write json: %v", err)
	}

	// Create ignored file
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("Ignore me"), 0644); err != nil {
		t.Fatalf("Failed to write ignored file: %v", err)
	}

	// Test loadChallenges
	challenges, err := loadChallenges(tmpDir)
	assert.NoError(t, err)
	assert.Len(t, challenges, 2)

	names := make(map[string]bool)
	for _, c := range challenges {
		names[c.Name] = true
	}

	assert.True(t, names["Challenge 1"], "Challenge 1 missing")
	assert.True(t, names["Challenge 2"], "Challenge 2 missing")
}
