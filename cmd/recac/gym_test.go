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

func (m *GymTestMockAgent) CalculateCost(model string, usage agent.TokenUsage) float64 {
	return 0.0
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

func TestLoadChallengesFile(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()

	// Test case 1: List of challenges
	listContent := `
- name: Challenge 1
  description: Desc 1
- name: Challenge 2
  description: Desc 2
`
	listPath := filepath.Join(tmpDir, "list.yaml")
	if err := os.WriteFile(listPath, []byte(listContent), 0644); err != nil {
		t.Fatal(err)
	}

	challenges, err := loadChallengesFile(listPath)
	if err != nil {
		t.Errorf("Failed to load list: %v", err)
	}
	if len(challenges) != 2 {
		t.Errorf("Expected 2 challenges, got %d", len(challenges))
	}

	// Test case 2: Single challenge
	singleContent := `
name: Challenge Single
description: Desc Single
`
	singlePath := filepath.Join(tmpDir, "single.yaml")
	if err := os.WriteFile(singlePath, []byte(singleContent), 0644); err != nil {
		t.Fatal(err)
	}

	challenges, err = loadChallengesFile(singlePath)
	if err != nil {
		t.Errorf("Failed to load single: %v", err)
	}
	if len(challenges) != 1 {
		t.Errorf("Expected 1 challenge, got %d", len(challenges))
	}

	// Test case 3: Directory
	dirPath := filepath.Join(tmpDir, "challenges_dir")
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatal(err)
	}
	// Add files to dir
	if err := os.WriteFile(filepath.Join(dirPath, "c1.yaml"), []byte(singleContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirPath, "c2.yaml"), []byte(listContent), 0644); err != nil {
		t.Fatal(err)
	}
	// Add non-yaml file
	if err := os.WriteFile(filepath.Join(dirPath, "readme.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatal(err)
	}

	challenges, err = loadChallenges(dirPath)
	if err != nil {
		t.Errorf("Failed to load directory: %v", err)
	}
	// 1 (single) + 2 (list) = 3
	if len(challenges) != 3 {
		t.Errorf("Expected 3 challenges, got %d", len(challenges))
	}
}
