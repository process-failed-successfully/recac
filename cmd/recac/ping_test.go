package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPingCmd_Success(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("provider", "openai")
	viper.Set("model", "gpt-4o")

	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(MockAgentClient)
	mockAgent.On("Send", mock.Anything, "Respond with exactly the word 'PONG'.").Return("PONG", nil)

	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		assert.Equal(t, "openai", provider)
		assert.Equal(t, "gpt-4o", model)
		return mockAgent, nil
	}

	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)

	pingCmd.SetOut(outBuf)
	pingCmd.SetErr(errBuf)
	// Directly call RunE to avoid cobra argument parsing issues during test
	err := pingCmd.RunE(pingCmd, []string{})
	require.NoError(t, err)

	outStr := outBuf.String()
	assert.Contains(t, outStr, "Testing connection to provider: openai (model: gpt-4o)...")
	assert.Contains(t, outStr, "✅ Connection successful!")
	assert.Contains(t, outStr, "Response: \"PONG\"")
	assert.Contains(t, outStr, "Latency:")

	mockAgent.AssertExpectations(t)
}

func TestPingCmd_AgentFailure(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("provider", "gemini")

	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(MockAgentClient)
	expectedErr := errors.New("API rate limit exceeded")
	mockAgent.On("Send", mock.Anything, "Respond with exactly the word 'PONG'.").Return("", expectedErr)

	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}

	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)

	pingCmd.SetOut(outBuf)
	pingCmd.SetErr(errBuf)

	err := pingCmd.RunE(pingCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API rate limit exceeded")

	errStr := errBuf.String()
	assert.Contains(t, errStr, "❌ Connection failed after")

	mockAgent.AssertExpectations(t)
}

func TestPingCmd_FactoryError(t *testing.T) {
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	expectedErr := errors.New("invalid provider")
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return nil, expectedErr
	}

	// Since we are running pingCmd.Execute(), it might run rootCmd instead if we aren't careful,
	// because `pingCmd` is added to `rootCmd` and we aren't explicitly setting the argument `ping`.
	// To execute just the ping command cleanly, we use executeCommand helper or call RunE directly.
	err := pingCmd.RunE(pingCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid provider")
}
