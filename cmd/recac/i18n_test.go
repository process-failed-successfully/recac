package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// I18nTestMockAgent is a mock implementation of agent.Agent
type I18nTestMockAgent struct {
	mock.Mock
}

func (m *I18nTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *I18nTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestI18nTranslateCmd(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "en.json")
	targetFile := filepath.Join(tmpDir, "es.json")

	// Create source file
	sourceData := map[string]interface{}{
		"hello": "Hello",
		"buy":   "Buy",
	}
	content, _ := json.Marshal(sourceData)
	os.WriteFile(sourceFile, content, 0644)

	// Mock factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := new(I18nTestMockAgent)
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// 1. Test fresh translation
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(`{ "hello": "Hola", "buy": "Comprar" }`, nil).Once()

	// Reset flags (since they are global)
	i18nTargetLang = ""
	i18nOutput = ""
	i18nForce = false

	// Execute command via rootCmd to ensure proper parsing
	// We need to make sure we don't trip over other global state
	rootCmd.SetArgs([]string{"i18n", "translate", sourceFile, "--target", "es", "--output", targetFile})

	// Capture output to prevent spamming test logs if needed, but for debugging let's leave it
	err := rootCmd.Execute()
	assert.NoError(t, err)

	// Verify output
	outContent, err := os.ReadFile(targetFile)
	assert.NoError(t, err)
	var outMap map[string]interface{}
	err = json.Unmarshal(outContent, &outMap)
	assert.NoError(t, err)
	assert.Equal(t, "Hola", outMap["hello"])
	assert.Equal(t, "Comprar", outMap["buy"])

	// 2. Test incremental translation
	// Add new key to source
	sourceData["bye"] = "Goodbye"
	content, _ = json.Marshal(sourceData)
	os.WriteFile(sourceFile, content, 0644)

	// Mock agent to only return the NEW key
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(`{ "bye": "Adios" }`, nil).Once()

	rootCmd.SetArgs([]string{"i18n", "translate", sourceFile, "--target", "es", "--output", targetFile})
	err = rootCmd.Execute()
	assert.NoError(t, err)

	outContent, _ = os.ReadFile(targetFile)
	json.Unmarshal(outContent, &outMap)
	assert.Equal(t, "Hola", outMap["hello"]) // Existing preserved
	assert.Equal(t, "Adios", outMap["bye"])   // New added

	// 3. Test nested structures
	sourceData["nested"] = map[string]interface{}{
		"deep": "Deep Value",
	}
	content, _ = json.Marshal(sourceData)
	os.WriteFile(sourceFile, content, 0644)

	mockAgent.On("Send", mock.Anything, mock.Anything).Return(`{ "nested": { "deep": "Valor Profundo" } }`, nil).Once()

	rootCmd.SetArgs([]string{"i18n", "translate", sourceFile, "--target", "es", "--output", targetFile})
	err = rootCmd.Execute()
	assert.NoError(t, err)

	outContent, _ = os.ReadFile(targetFile)
	json.Unmarshal(outContent, &outMap)
	if nested, ok := outMap["nested"].(map[string]interface{}); ok {
		assert.Equal(t, "Valor Profundo", nested["deep"])
	} else {
		t.Error("nested key is missing or not a map")
	}
}
