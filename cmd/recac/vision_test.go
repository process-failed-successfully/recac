package main

import (
	"bytes"
	"context"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockVisionAgentClient is a mock implementation of agent.VisionAgent
type MockVisionAgentClient struct {
	mock.Mock
}

func (m *MockVisionAgentClient) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockVisionAgentClient) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt, callback)
	callback(args.String(0))
	return args.String(0), args.Error(1)
}

func (m *MockVisionAgentClient) SendImage(ctx context.Context, prompt string, imagePath string) (string, error) {
	args := m.Called(ctx, prompt, imagePath)
	return args.String(0), args.Error(1)
}

// MockAgentClientNoVision is a mock implementation of agent.Agent (without Vision)
type MockAgentClientNoVision struct {
	mock.Mock
}

func (m *MockAgentClientNoVision) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockAgentClientNoVision) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt, callback)
	return args.String(0), args.Error(1)
}

func TestVisionCmd(t *testing.T) {
	// Setup
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(MockVisionAgentClient)
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}

	expectedResponse := "A beautiful sunset."
	imagePath := "test.jpg"

	mockAgent.On("SendImage", mock.Anything, "Describe this image.", imagePath).Return(expectedResponse, nil)

	// Execute
	cmd := &cobra.Command{Use: "vision", RunE: runVisionCmd}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	viper.Set("provider", "test-provider")
	viper.Set("model", "test-model")

	cmd.SetArgs([]string{imagePath})

	err := cmd.Execute()
	assert.NoError(t, err)

	assert.Contains(t, buf.String(), "Analyzing image...")
	assert.Contains(t, buf.String(), expectedResponse)

	mockAgent.AssertExpectations(t)
}

func TestVisionCmd_WithPrompt(t *testing.T) {
	// Setup
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(MockVisionAgentClient)
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}

	expectedResponse := "It is a red car."
	imagePath := "car.jpg"
	customPrompt := "What color is the car?"

	mockAgent.On("SendImage", mock.Anything, customPrompt, imagePath).Return(expectedResponse, nil)

	cmd := &cobra.Command{Use: "vision", RunE: runVisionCmd}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{imagePath, customPrompt})

	err := cmd.Execute()
	assert.NoError(t, err)

	assert.Contains(t, buf.String(), expectedResponse)
	mockAgent.AssertExpectations(t)
}

func TestVisionCmd_NoVisionSupport(t *testing.T) {
	// Setup
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(MockAgentClientNoVision)
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}

	cmd := &cobra.Command{Use: "vision", RunE: runVisionCmd}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"image.jpg"})

	viper.Set("provider", "simple-agent")

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support vision capabilities")
}
