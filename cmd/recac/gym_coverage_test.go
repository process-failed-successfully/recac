package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRunGymSession_DockerInitError(t *testing.T) {
	// Setup
	originalDockerFactory := gymDockerClientFactory
	defer func() { gymDockerClientFactory = originalDockerFactory }()

	gymDockerClientFactory = func(project string) (runner.DockerClient, error) {
		return nil, errors.New("docker init failed")
	}

	// Execute
	_, err := runGymSession(context.Background(), GymChallenge{Name: "Test"})

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to init docker")
}

func TestRunGymSession_AgentInitError(t *testing.T) {
	// Setup
	originalDockerFactory := gymDockerClientFactory
	originalAgentFactory := gymAgentFactory
	defer func() {
		gymDockerClientFactory = originalDockerFactory
		gymAgentFactory = originalAgentFactory
	}()

	mockDocker := new(GymMockDockerClient)
	gymDockerClientFactory = func(project string) (runner.DockerClient, error) {
		return mockDocker, nil
	}

	gymAgentFactory = func(provider, apiKey, model, workspace, projectID string) (agent.Agent, error) {
		return nil, errors.New("agent init failed")
	}

	// Execute
	_, err := runGymSession(context.Background(), GymChallenge{Name: "Test"})

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to init agent")
}

func TestRunGymSession_SessionStartError(t *testing.T) {
	// Setup
	originalDockerFactory := gymDockerClientFactory
	originalAgentFactory := gymAgentFactory
	originalSessionFactory := gymSessionFactory
	defer func() {
		gymDockerClientFactory = originalDockerFactory
		gymAgentFactory = originalAgentFactory
		gymSessionFactory = originalSessionFactory
	}()

	mockDocker := new(GymMockDockerClient)
	gymDockerClientFactory = func(project string) (runner.DockerClient, error) {
		return mockDocker, nil
	}

	mockAgent := new(GymMockAgent) // Reuse from gym_test.go
	gymAgentFactory = func(provider, apiKey, model, workspace, projectID string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Mock Session Factory to return a session that uses our mock docker
	gymSessionFactory = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		return runner.NewSession(d, a, workspace, image, project, provider, model, maxAgents)
	}

	// Expect Start to fail
	// Session.Start calls Docker.RunContainer
	mockDocker.On("CheckDaemon", mock.Anything).Return(nil)
	mockDocker.On("ImageExists", mock.Anything, mock.Anything).Return(true, nil)
	// Make RunContainer fail
	mockDocker.On("RunContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("container start failed"))

	// Execute
	_, err := runGymSession(context.Background(), GymChallenge{Name: "Test"})

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start session")
}

func TestLoadChallenges_Error(t *testing.T) {
	// Non-existent file
	_, err := loadChallenges("non-existent.yaml")
	assert.Error(t, err)

	// Invalid YAML
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.yaml")
	os.WriteFile(invalidPath, []byte(": invalid yaml"), 0644)

	_, err = loadChallenges(invalidPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse challenges file")
}

func TestLoadChallenges_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid yaml file
	validPath := filepath.Join(tmpDir, "challenge.yaml")
	validContent := `
- name: Test Challenge
  description: Test Description
  language: python
  tests: |
    def test_foo(): assert True
  test_file: test_foo.py
  timeout: 10
`
	err := os.WriteFile(validPath, []byte(validContent), 0644)
	assert.NoError(t, err)

	// Create a subdirectory with another file
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	assert.NoError(t, err)

	subPath := filepath.Join(subDir, "sub.yaml")
	err = os.WriteFile(subPath, []byte(validContent), 0644)
	assert.NoError(t, err)

	// Create a non-yaml file
	ignorePath := filepath.Join(tmpDir, "ignore.txt")
	err = os.WriteFile(ignorePath, []byte("ignore"), 0644)
	assert.NoError(t, err)

	// Mock readFileFunc for the files we created?
	// loadChallenges uses loadChallengesFile which uses readFileFunc.
	// Since we wrote real files, we should make readFileFunc read real files.
	originalReadFile := readFileFunc
	defer func() { readFileFunc = originalReadFile }()
	readFileFunc = os.ReadFile // Use real file reading

	challenges, err := loadChallenges(tmpDir)
	assert.NoError(t, err)
	assert.Len(t, challenges, 2)
}

func TestRunGymSession_WriteFileError(t *testing.T) {
	// Setup
	originalDockerFactory := gymDockerClientFactory
	originalAgentFactory := gymAgentFactory
	originalSessionFactory := gymSessionFactory
	originalWriteFile := writeFileFunc
	defer func() {
		gymDockerClientFactory = originalDockerFactory
		gymAgentFactory = originalAgentFactory
		gymSessionFactory = originalSessionFactory
		writeFileFunc = originalWriteFile
	}()

	mockDocker := new(GymMockDockerClient)
	gymDockerClientFactory = func(project string) (runner.DockerClient, error) {
		return mockDocker, nil
	}

	mockAgent := new(GymMockAgent)
	gymAgentFactory = func(provider, apiKey, model, workspace, projectID string) (agent.Agent, error) {
		return mockAgent, nil
	}

	gymSessionFactory = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		return runner.NewSession(d, a, workspace, image, project, provider, model, maxAgents)
	}

	// Mock writeFileFunc to fail
	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		return errors.New("write failed")
	}

	// Challenge with tests to trigger write
	challenge := GymChallenge{
		Name:     "Test",
		Tests:    "test content",
		TestFile: "test.py",
	}

	// Execute
	_, err := runGymSession(context.Background(), challenge)

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write test file")
}
