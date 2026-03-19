package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
)

type mockAIAgent struct {
	response string
	err      error
}

func (m *mockAIAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockAIAgent) SendStream(ctx context.Context, prompt string, streamFunc func(string)) (string, error) {
	return "", nil
}

func (m *mockAIAgent) Chat(ctx context.Context, messages []agent.Message) (string, error) {
	return "", nil
}

func TestGeneratePipelineYAML_Success(t *testing.T) {
	// Save the original function and restore it later
	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	expectedYAML := `name: test-pipeline
jobs:
  test-job:
    summary: Test Job
    task: Do something`

	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return &mockAIAgent{
			response: "```yaml\n" + expectedYAML + "\n```",
		}, nil
	}

	pipelineYAML, err := GeneratePipelineYAML(context.Background(), "create a test job", "provider", "model", "key")
	assert.NoError(t, err)
	assert.Equal(t, expectedYAML, pipelineYAML)
}

func TestGeneratePipelineYAML_AgentInitError(t *testing.T) {
	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return nil, errors.New("init error")
	}

	_, err := GeneratePipelineYAML(context.Background(), "prompt", "provider", "model", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "init error")
}

func TestGeneratePipelineYAML_SendError(t *testing.T) {
	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return &mockAIAgent{
			err: errors.New("send error"),
		}, nil
	}

	_, err := GeneratePipelineYAML(context.Background(), "prompt", "provider", "model", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "send error")
}

func TestGeneratePipelineYAML_NoMarkdownBlocks(t *testing.T) {
	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	expectedYAML := `name: no-markdown
jobs:
  job-1:
    summary: Job 1`

	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return &mockAIAgent{
			response: expectedYAML,
		}, nil
	}

	pipelineYAML, err := GeneratePipelineYAML(context.Background(), "prompt", "provider", "model", "key")
	assert.NoError(t, err)
	assert.Equal(t, expectedYAML, strings.TrimSpace(pipelineYAML))
}
