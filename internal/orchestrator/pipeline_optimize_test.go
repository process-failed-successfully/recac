package orchestrator

import (
	"context"
	"errors"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptimizePipelineYAML_Success(t *testing.T) {
	oldAgentFunc := newAgentFunc
	defer func() { newAgentFunc = oldAgentFunc }()

	expectedYAML := `name: optimized-pipeline
jobs:
  test-job:
    summary: Optimized Job
    task: Do something optimized`

	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return &mockAIAgent{
			response: "```yaml\n" + expectedYAML + "\n```",
		}, nil
	}

	ctx := context.Background()
	originalYAML := "name: test\njobs:\n  a:\n    summary: a\n"

	result, err := OptimizePipelineYAML(ctx, originalYAML, "mock-provider", "mock-model", "mock-key")

	require.NoError(t, err)
	assert.Equal(t, expectedYAML, result)
}

func TestOptimizePipelineYAML_InitializationError(t *testing.T) {
	oldAgentFunc := newAgentFunc
	defer func() { newAgentFunc = oldAgentFunc }()

	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return nil, errors.New("failed to init ai agent")
	}

	ctx := context.Background()
	_, err := OptimizePipelineYAML(ctx, "test yaml", "mock-provider", "mock-model", "mock-key")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize AI agent")
}

func TestOptimizePipelineYAML_SendError(t *testing.T) {
	oldAgentFunc := newAgentFunc
	defer func() { newAgentFunc = oldAgentFunc }()

	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return &mockAIAgent{
			err: errors.New("ai failure"),
		}, nil
	}

	ctx := context.Background()
	_, err := OptimizePipelineYAML(ctx, "test yaml", "mock-provider", "mock-model", "mock-key")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to optimize pipeline with AI")
}

func TestOptimizePipelineYAML_CleanupMarkdown(t *testing.T) {
	oldAgentFunc := newAgentFunc
	defer func() { newAgentFunc = oldAgentFunc }()

	newAgentFunc = func(provider, apiKey, model, customURL, systemPrompt string) (agent.Agent, error) {
		return &mockAIAgent{
			response: "```yaml\nname: test\njobs:\n  a:\n    summary: a\n```",
		}, nil
	}

	ctx := context.Background()
	result, err := OptimizePipelineYAML(ctx, "test yaml", "mock-provider", "mock-model", "mock-key")

	require.NoError(t, err)
	assert.Equal(t, "name: test\njobs:\n  a:\n    summary: a", result)
}

func TestOptimizePipelineYAML_CleanupMarkdown_PlainCodeBlock(t *testing.T) {
	oldAgentFunc := newAgentFunc
	defer func() { newAgentFunc = oldAgentFunc }()

	newAgentFunc = func(provider, apiKey, model, customURL, systemPrompt string) (agent.Agent, error) {
		return &mockAIAgent{
			response: "```\nname: test\njobs:\n  a:\n    summary: a\n```",
		}, nil
	}

	ctx := context.Background()
	result, err := OptimizePipelineYAML(ctx, "test yaml", "mock-provider", "mock-model", "mock-key")

	require.NoError(t, err)
	assert.Equal(t, "name: test\njobs:\n  a:\n    summary: a", result)
}

func TestOptimizePipelineYAML_CleanupMarkdown_NoCodeBlock(t *testing.T) {
	oldAgentFunc := newAgentFunc
	defer func() { newAgentFunc = oldAgentFunc }()

	newAgentFunc = func(provider, apiKey, model, customURL, systemPrompt string) (agent.Agent, error) {
		return &mockAIAgent{
			response: "name: test\njobs:\n  a:\n    summary: a\n",
		}, nil
	}

	ctx := context.Background()
	result, err := OptimizePipelineYAML(ctx, "test yaml", "mock-provider", "mock-model", "mock-key")

	require.NoError(t, err)
	assert.Equal(t, "name: test\njobs:\n  a:\n    summary: a", result)
}
