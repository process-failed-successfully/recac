package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type I18nTestMockAgent struct {
	Response string
}

func (m *I18nTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *I18nTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestI18nCmd(t *testing.T) {
	// Setup Temp Dir
	tmpDir := t.TempDir()

	// Create en.json
	enContent := map[string]interface{}{
		"hello": "Hello World",
		"bye":   "Goodbye",
		"nested": map[string]interface{}{
			"title": "My Title",
		},
	}
	enBytes, _ := json.Marshal(enContent)
	err := os.WriteFile(filepath.Join(tmpDir, "en.json"), enBytes, 0644)
	require.NoError(t, err)

	// Create es.json (missing keys)
	esContent := map[string]interface{}{
		"bye": "Adiós",
	}
	esBytes, _ := json.Marshal(esContent)
	err = os.WriteFile(filepath.Join(tmpDir, "es.json"), esBytes, 0644)
	require.NoError(t, err)

	// Mock Agent
	expectedTranslation := map[string]interface{}{
		"hello":        "Hola Mundo",
		"nested.title": "Mi Título",
	}
	transBytes, _ := json.Marshal(expectedTranslation)

	mockAgent := &I18nTestMockAgent{
		Response: string(transBytes),
	}

	// Swap Factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Run Command
	// We use --yes to avoid prompt and specify source
	rootCmd.SetArgs([]string{"i18n", tmpDir, "--source", "en.json", "--yes"})

	// Capture output?
	// The command writes to stdout using cmd.OutOrStdout(), but our test runner doesn't capture it easily unless we set it.
	// We rely on file changes.

	err = rootCmd.Execute()
	require.NoError(t, err)

	// Verify es.json
	esReadBytes, err := os.ReadFile(filepath.Join(tmpDir, "es.json"))
	require.NoError(t, err)

	var esResult map[string]interface{}
	err = json.Unmarshal(esReadBytes, &esResult)
	require.NoError(t, err)

	assert.Equal(t, "Adiós", esResult["bye"])        // Existing
	assert.Equal(t, "Hola Mundo", esResult["hello"]) // Translated

	// Check nested
	// Our flat structure logic: "nested.title" from translation should be unflattened
	nested, ok := esResult["nested"].(map[string]interface{})
	assert.True(t, ok, "nested key should be a map")
	if ok {
		assert.Equal(t, "Mi Título", nested["title"])
	}
}

func TestI18nCmd_Verify(t *testing.T) {
	// Setup Temp Dir
	tmpDir := t.TempDir()

	// Create en.json
	enContent := map[string]interface{}{"foo": "bar"}
	enBytes, _ := json.Marshal(enContent)
	os.WriteFile(filepath.Join(tmpDir, "en.json"), enBytes, 0644)

	// Create es.json (empty)
	os.WriteFile(filepath.Join(tmpDir, "es.json"), []byte("{}"), 0644)

	// Run Command with --verify
	rootCmd.SetArgs([]string{"i18n", tmpDir, "--source", "en.json", "--verify"})

	err := rootCmd.Execute()
	assert.Error(t, err) // Should fail because missing keys
}
