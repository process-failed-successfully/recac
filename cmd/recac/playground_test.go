package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"recac/internal/agent"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// MockPlaygroundDockerClient
type MockPlaygroundDockerClient struct {
	RunContainerFunc    func(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, cmd []string, user string) (string, error)
	WaitContainerFunc   func(ctx context.Context, containerID string) (int64, error)
	ContainerLogsFunc   func(ctx context.Context, containerID string) (io.ReadCloser, error)
	RemoveContainerFunc func(ctx context.Context, containerID string, force bool) error
	CloseFunc           func() error
}

func (m *MockPlaygroundDockerClient) RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, cmd []string, user string) (string, error) {
	if m.RunContainerFunc != nil {
		return m.RunContainerFunc(ctx, imageRef, workspace, extraBinds, env, cmd, user)
	}
	return "mock-container-id", nil
}

func (m *MockPlaygroundDockerClient) WaitContainer(ctx context.Context, containerID string) (int64, error) {
	if m.WaitContainerFunc != nil {
		return m.WaitContainerFunc(ctx, containerID)
	}
	return 0, nil
}

func (m *MockPlaygroundDockerClient) ContainerLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	if m.ContainerLogsFunc != nil {
		return m.ContainerLogsFunc(ctx, containerID)
	}
	return io.NopCloser(strings.NewReader("Mock Output")), nil
}

func (m *MockPlaygroundDockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	if m.RemoveContainerFunc != nil {
		return m.RemoveContainerFunc(ctx, containerID, force)
	}
	return nil
}

func (m *MockPlaygroundDockerClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// MockPlaygroundAgent
type MockPlaygroundAgent struct {
	agent.Agent
	SendFunc func(ctx context.Context, content string) (string, error)
}

func (m *MockPlaygroundAgent) Send(ctx context.Context, content string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, content)
	}
	return "Mock AI Response", nil
}

func TestPlayground_InitialModel(t *testing.T) {
	m := initialPlaygroundModel()
	assert.Equal(t, langGo, m.language)
	assert.Equal(t, modeCode, m.mode)
	assert.Contains(t, m.codeArea.Value(), "Hello from Playground")
}

func TestPlayground_SwitchLanguage(t *testing.T) {
	m := initialPlaygroundModel()

	// Switch to Python
	m, cmd := updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlL})
	assert.Nil(t, cmd) // No cmd for switch
	assert.Equal(t, langPython, m.language)
	assert.Contains(t, m.codeArea.Value(), "print")

	// Switch to JS
	m, cmd = updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlL})
	assert.Equal(t, langJS, m.language)
	assert.Contains(t, m.codeArea.Value(), "console.log")
}

func TestPlayground_RunCode(t *testing.T) {
	// Setup Mocks
	originalDockerFactory := playgroundDockerFactory
	defer func() { playgroundDockerFactory = originalDockerFactory }()

	playgroundDockerFactory = func(project string) (PlaygroundDockerClient, error) {
		return &MockPlaygroundDockerClient{
			ContainerLogsFunc: func(ctx context.Context, containerID string) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("Execution Success")), nil
			},
		}, nil
	}

	m := initialPlaygroundModel()

	// Trigger Run (Ctrl+R)
	m, cmd := updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlR})
	assert.True(t, m.running)
	assert.NotNil(t, cmd)

	// Execute Cmd
	msg := cmd()
	runMsg, ok := msg.(runCodeMsg)
	assert.True(t, ok)
	assert.NoError(t, runMsg.err)
	assert.Equal(t, "Execution Success", runMsg.output)

	// Update with Result
	m, _ = updateModel(m, runMsg)
	assert.False(t, m.running)
	assert.Equal(t, "Execution Success", m.outputContent)
}

func TestPlayground_AskAI(t *testing.T) {
	// Setup Mocks
	originalAgentFactory := agentClientFactory
	defer func() { agentClientFactory = originalAgentFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, cwd, persona string) (agent.Agent, error) {
		return &MockPlaygroundAgent{
			SendFunc: func(ctx context.Context, content string) (string, error) {
				return "Fixed code", nil
			},
		}, nil
	}

	m := initialPlaygroundModel()
	m.mode = modeAI
	m.aiInput.SetValue("Fix this")

	// Trigger Enter
	m, cmd := updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, m.thinking)
	assert.NotNil(t, cmd)

	// Execute Cmd
	msg := cmd()
	aiMsg, ok := msg.(aiResponseMsg)
	assert.True(t, ok)
	assert.NoError(t, aiMsg.err)
	assert.Equal(t, "Fixed code", aiMsg.response)

	// Update with Result
	m, _ = updateModel(m, aiMsg)
	assert.False(t, m.thinking)
	assert.Contains(t, m.chatHistory, "Fixed code")
}

// Helper to cast Tea Model to our model
func updateModel(m playgroundModel, msg tea.Msg) (playgroundModel, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(playgroundModel), cmd
}
