package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/flashcards"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// MockFlashcardsAgent
type MockFlashcardsAgent struct {
	Response string
}

func (m *MockFlashcardsAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockFlashcardsAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestFlashcardsGenerate(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// 2. Mock Agent Factory
	originalAgentFactory := agentClientFactory
	defer func() { agentClientFactory = originalAgentFactory }()

	mockResponse := `[
		{"question": "What is A?", "answer": "B", "topic": "test"},
		{"question": "What is C?", "answer": "D", "topic": "test"}
	]`

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockFlashcardsAgent{Response: mockResponse}, nil
	}

	// 3. Mock Context Generation
	originalContextFunc := generateContextFunc
	defer func() { generateContextFunc = originalContextFunc }()

	generateContextFunc = func(opts ContextOptions) (string, error) {
		return "mock codebase context", nil
	}

	// 4. Run Command
	cmd := &cobra.Command{Use: "generate", RunE: runFlashcardsGenerate}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Set flags
	flashcardsTopic = "test"
	flashcardsLimit = 2
	flashcardsFocus = "."

	err := runFlashcardsGenerate(cmd, []string{})
	assert.NoError(t, err)

	// 5. Verify Output
	assert.Contains(t, buf.String(), "Generated and saved 2 flashcards")

	// 6. Verify Store
	storePath := filepath.Join(tmpDir, ".recac", "flashcards.json")
	assert.FileExists(t, storePath)

	content, _ := os.ReadFile(storePath)
	var cards []flashcards.Flashcard
	json.Unmarshal(content, &cards)
	assert.Len(t, cards, 2)
	assert.Equal(t, "What is A?", cards[0].Question)
}

func TestFlashcardsStats(t *testing.T) {
	// 1. Setup Temp Dir and Store
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	storePath := filepath.Join(tmpDir, ".recac", "flashcards.json")
	os.MkdirAll(filepath.Dir(storePath), 0755)

	cards := []flashcards.Flashcard{
		flashcards.NewFlashcard("Q1", "A1", "", "t"),
		flashcards.NewFlashcard("Q2", "A2", "", "t"),
	}
	// Modify one card to be "Learning"
	cards[1].State = flashcards.StateLearning

	data, _ := json.Marshal(cards)
	os.WriteFile(storePath, data, 0644)

	// 2. Run Command
	cmd := &cobra.Command{Use: "stats", RunE: runFlashcardsStats}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runFlashcardsStats(cmd, []string{})
	assert.NoError(t, err)

	// 3. Verify Output
	output := buf.String()
	assert.Contains(t, output, "Total Cards:   2")
	assert.Contains(t, output, "New:           1")
	assert.Contains(t, output, "Learning:      1")
}
