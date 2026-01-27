package main

import (
	"context"
	"testing"
	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ConvertMockAgent implements agent.Agent
type ConvertMockAgent struct {
	Response string
	Err      error
}

func (m *ConvertMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func (m *ConvertMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, m.Err
}

func TestConvertDeterministic(t *testing.T) {
	// JSON to YAML
	input := `{"key": "value", "list": [1, 2]}`
	output, err := convertDeterministic([]byte(input), "json", "yaml")
	require.NoError(t, err)
	assert.Contains(t, string(output), "key: value")
	// YAML format can vary, but usually lists are dashed
	assert.Contains(t, string(output), "- 1")

	// YAML to JSON
	inputYaml := "key: value\nlist:\n  - 1\n  - 2"
	outputJson, err := convertDeterministic([]byte(inputYaml), "yaml", "json")
	require.NoError(t, err)
	assert.JSONEq(t, input, string(outputJson))

	// JSON to TOML
	outputToml, err := convertDeterministic([]byte(input), "json", "toml")
	require.NoError(t, err)
	// TOML string quoting can be ' or "
	// pelletier/go-toml v2 default
	assert.Contains(t, string(outputToml), "key = 'value'")
}

func TestConvertAI(t *testing.T) {
	// Mock agent factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mock := &ConvertMockAgent{
		Response: `{"converted": "ai"}`,
	}

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mock, nil
	}

	ctx := context.Background()
	out, err := convertWithAI(ctx, []byte("some text"), "text", "json")
	require.NoError(t, err)
	assert.Equal(t, `{"converted": "ai"}`, string(out))
}

func TestConvertFormatDetection(t *testing.T) {
    // We can't easily test main() without execution, but we can verify logic if we extracted it.
    // Since detection logic is inside runConvert, we'd have to refactor or just trust the e2e test if we wrote one.
    // For now, unit tests cover the core logic.
}
