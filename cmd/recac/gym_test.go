package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"log/slog"
	"recac/internal/agent"
	"recac/internal/docker"
	"recac/internal/notify"
	"recac/internal/runner"
	"recac/internal/telemetry"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock objects
type GymMockDockerClient struct {
	mock.Mock
}

func (m *GymMockDockerClient) CheckDaemon(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *GymMockDockerClient) RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error) {
	args := m.Called(ctx, imageRef, workspace, extraBinds, env, user)
	return args.String(0), args.Error(1)
}

func (m *GymMockDockerClient) StopContainer(ctx context.Context, containerID string) error {
	args := m.Called(ctx, containerID)
	return args.Error(0)
}

func (m *GymMockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	args := m.Called(ctx, containerID, cmd)
	return args.String(0), args.Error(1)
}

func (m *GymMockDockerClient) ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error) {
	args := m.Called(ctx, containerID, user, cmd)
	return args.String(0), args.Error(1)
}

func (m *GymMockDockerClient) ImageExists(ctx context.Context, tag string) (bool, error) {
	args := m.Called(ctx, tag)
	return args.Bool(0), args.Error(1)
}

func (m *GymMockDockerClient) ImageBuild(ctx context.Context, opts docker.ImageBuildOptions) (string, error) {
	args := m.Called(ctx, opts)
	return args.String(0), args.Error(1)
}

func (m *GymMockDockerClient) PullImage(ctx context.Context, imageRef string) error {
	args := m.Called(ctx, imageRef)
	return args.Error(0)
}

type GymMockAgent struct {
	mock.Mock
}

func (m *GymMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *GymMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestLoadChallenges(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	yamlContent := `
- name: Test Challenge
  description: Test Description
  language: python
  tests: |
    def test_foo(): assert True
  test_file: test_foo.py
  timeout: 10
`
	yamlPath := filepath.Join(tmpDir, "challenges.yaml")
	err := os.WriteFile(yamlPath, []byte(yamlContent), 0644)
	assert.NoError(t, err)

	challenges, err := loadChallenges(yamlPath)
	assert.NoError(t, err)
	assert.Len(t, challenges, 1)
	assert.Equal(t, "Test Challenge", challenges[0].Name)
	assert.Equal(t, "Test Description", challenges[0].Description)
}

func TestRunGym(t *testing.T) {
	// Mock runGymSessionFunc
	originalRunGymSessionFunc := runGymSessionFunc
	defer func() { runGymSessionFunc = originalRunGymSessionFunc }()

	runGymSessionFunc = func(ctx context.Context, challenge GymChallenge) (*GymResult, error) {
		if challenge.Name == "Fail Challenge" {
			return nil, errors.New("simulated error")
		}
		return &GymResult{
			Challenge: challenge.Name,
			Passed:    true,
			Output:    "Success",
			Duration:  time.Second,
			Cost:      0.01,
		}, nil
	}

	// Create a temp challenges file
	tmpDir := t.TempDir()
	yamlContent := `
- name: Success Challenge
  description: desc
- name: Fail Challenge
  description: desc
`
	yamlPath := filepath.Join(tmpDir, "challenges.yaml")
	err := os.WriteFile(yamlPath, []byte(yamlContent), 0644)
	assert.NoError(t, err)

	cmd := &cobra.Command{}
	err = runGym(cmd, []string{yamlPath})
	assert.NoError(t, err)
	// We verify output by checking no error return. Reporting logic prints to stdout.
}

func TestRunGymSession(t *testing.T) {
	// Mock factories
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

	mockAgent := new(GymMockAgent)
	gymAgentFactory = func(provider, apiKey, model, workspace, projectID string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Mock Session creation to return a safe session without side effects (DB, etc)
	gymSessionFactory = func(d runner.DockerClient, a agent.Agent, workspace, image, project, provider, model string, maxAgents int) *runner.Session {
		return &runner.Session{
			Docker:        d,
			Agent:         a,
			Workspace:     workspace,
			Image:         image,
			Project:       project,
			MaxIterations: 1,
			Notifier:      notify.NewManager(telemetry.LogInfof),
			Logger:        slog.Default(),
			SleepFunc:     func(d time.Duration) {},
		}
	}

	// Challenge data
	challenge := GymChallenge{
		Name:     "Test",
		Language: "python",
		TestFile: "test.py",
		Tests:    "print('hello')",
	}

	// Expectations
	mockDocker.On("CheckDaemon", mock.Anything).Return(nil)
	mockDocker.On("ImageExists", mock.Anything, mock.Anything).Return(true, nil)
	mockDocker.On("RunContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("mock-container-id", nil)
	mockDocker.On("StopContainer", mock.Anything, "mock-container-id").Return(nil)

	// Agent expectations
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("I'm working on it...", nil).Maybe()

	// Setup calls (passwd, git, etc) - allow any Exec/ExecAsUser calls generally
	// But match specific verification call specifically if needed.
	// Since verification is the last step, we can return "OK" for the python test, and "" for others.
	// However, testify mock matches in order or specific args.

	// Helper to match verification command
	isVerificationCmd := func(cmd []string) bool {
		return len(cmd) > 1 && cmd[0] == "python3" && cmd[1] == "test.py"
	}

	// Verification call
	mockDocker.On("Exec", mock.Anything, "mock-container-id", mock.MatchedBy(isVerificationCmd)).Return("OK", nil)

	// Other Exec calls (setup)
	mockDocker.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return("", nil).Maybe()
	mockDocker.On("ExecAsUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", nil).Maybe()

	// Run
	ctx := context.Background()
	res, err := runGymSession(ctx, challenge)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, res.Passed)
	assert.Equal(t, "OK", res.Output)
}
