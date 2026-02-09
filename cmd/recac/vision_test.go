package main

import (
	"context"
	"os"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// MockVisionAgentClient wraps MockAgent to capture calls
type MockVisionAgentClient struct {
	*agent.MockAgent
	lastPrompt string
	lastImage  string
}

func (m *MockVisionAgentClient) SendImage(ctx context.Context, prompt string, imagePath string) (string, error) {
	m.lastPrompt = prompt
	m.lastImage = imagePath
	return "Mock vision response", nil
}

// MockTextOnlyAgent only implements Agent, not VisionAgent
type MockTextOnlyAgent struct{}

func (m *MockTextOnlyAgent) Send(ctx context.Context, prompt string) (string, error) { return "text", nil }
func (m *MockTextOnlyAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return "text", nil
}

func TestVisionCommand(t *testing.T) {
	// Create a dummy image file
	tmpFile, err := os.CreateTemp("", "test_image.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Mock the agent factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := &MockVisionAgentClient{
		MockAgent: agent.NewMockAgent(),
	}

	// Reset viper
	viper.Reset()

	t.Run("Success", func(t *testing.T) {
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockAgent, nil
		}
		viper.Set("provider", "mock")

		// We can call RunE directly to avoid Cobra's global state issues with flags/args
		err := runVision(visionCmd, []string{tmpFile.Name(), "Test prompt"})
		assert.NoError(t, err)
		assert.Equal(t, "Test prompt", mockAgent.lastPrompt)
		assert.Equal(t, tmpFile.Name(), mockAgent.lastImage)
	})

	t.Run("DefaultPrompt", func(t *testing.T) {
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockAgent, nil
		}
		err := runVision(visionCmd, []string{tmpFile.Name()})
		assert.NoError(t, err)
		assert.Equal(t, "Describe this image.", mockAgent.lastPrompt)
	})

	t.Run("FileNotFound", func(t *testing.T) {
		err := runVision(visionCmd, []string{"nonexistent.jpg"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "image file not found")
	})

	t.Run("NotVisionAgent", func(t *testing.T) {
		// Mock a non-vision agent
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockTextOnlyAgent{}, nil
		}

		err := runVision(visionCmd, []string{tmpFile.Name()})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not support vision")
	})
}
