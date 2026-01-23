package main

import (
	"bytes"
	"context"
	"errors"
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

func TestRunGymCommand(t *testing.T) {
	// 1. Create Temp Config
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "challenges.yaml")
	configContent := `
- name: Test Challenge
  description: Implement hello world
  language: python
  tests: print("Hello")
  test_file: test_hello.py
  timeout: 5
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	// 2. Setup Mocks
	mockDocker := &GymTestMockDockerClient{
		execOutput: "PASSED",
	}
	mockAgent := &GymTestMockAgent{
		response: "Completed",
	}

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

	// 3. Run Command
	buf := new(bytes.Buffer)
	// We need to execute via rootCmd because gymCmd is attached to it in init()
	// and calling gymCmd.Execute() triggers the root command anyway.
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"gym", configFile})

	err = rootCmd.Execute()

	// 4. Verify
	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "Loaded 1 challenges")
	assert.Contains(t, output, "Running challenge: Test Challenge")
	assert.Contains(t, output, "PASS 🟢")
	assert.Contains(t, output, "Summary: 1/1 passed")
}

func TestLoadChallenges_Error(t *testing.T) {
	// Test file not found
	_, err := loadChallenges("non_existent_file.yaml")
	assert.Error(t, err)

	// Test invalid yaml
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.yaml")
	err = os.WriteFile(badFile, []byte("invalid: : yaml"), 0644)
	assert.NoError(t, err)

	_, err = loadChallenges(badFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse challenges file")

	// Test directory
	dir := filepath.Join(tmpDir, "some_dir")
	os.Mkdir(dir, 0755)
	_, err = loadChallenges(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "directory loading not implemented yet")
}

func TestRunGymSession_Failure(t *testing.T) {
	// Setup Mocks for Failure
	mockDocker := &GymTestMockDockerClient{
		execOutput: "FAILED",
		execErr:    errors.New("command failed"),
	}
	mockAgent := &GymTestMockAgent{
		response: "I tried.",
	}

	originalDockerFactory := gymDockerClientFactory
	originalAgentFactory := gymAgentFactory
	defer func() {
		gymDockerClientFactory = originalDockerFactory
		gymAgentFactory = originalAgentFactory
	}()

	gymDockerClientFactory = func(project string) (runner.DockerClient, error) {
		return mockDocker, nil
	}
	gymAgentFactory = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
		return mockAgent, nil
	}

	challenge := GymChallenge{
		Name:     "Fail Challenge",
		Language: "python",
		TestFile: "fail.py",
	}

	result, err := runGymSession(context.Background(), challenge)
	assert.NoError(t, err) // runGymSession swallows exec error but returns passed=false
	assert.False(t, result.Passed)
}
