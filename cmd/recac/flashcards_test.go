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
	"time"

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

	// Since map iteration is random, the order in file (and thus loaded slice) is not guaranteed
	// We check that "What is A?" exists in the list
	foundA := false
	for _, c := range cards {
		if c.Question == "What is A?" {
			foundA = true
			break
		}
	}
	assert.True(t, foundA, "Expected to find card with question 'What is A?'")
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

func TestFlashcardsList(t *testing.T) {
	// 1. Setup Temp Dir and Store
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	storePath := filepath.Join(tmpDir, ".recac", "flashcards.json")
	os.MkdirAll(filepath.Dir(storePath), 0755)

	// Create some cards
	now := time.Now()
	cards := []flashcards.Flashcard{
		{
			ID:       "1",
			Question: "Q1",
			Answer:   "A1",
			Topic:    "T1",
			DueDate:  now,
			State:    flashcards.StateNew,
		},
		{
			ID:       "2",
			Question: "Q2",
			Answer:   "A2",
			Topic:    "T2",
			DueDate:  now.Add(24 * time.Hour),
			State:    flashcards.StateLearning,
		},
	}

	data, _ := json.Marshal(cards)
	os.WriteFile(storePath, data, 0644)

	// 2. Run Command
	cmd := &cobra.Command{Use: "list", RunE: runFlashcardsList}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runFlashcardsList(cmd, []string{})
	assert.NoError(t, err)

	// 3. Verify Output
	output := buf.String()
	assert.Contains(t, output, "[T1] Q1")
	assert.Contains(t, output, "[T2] Q2")

	// Check date format if needed (just check it appears)
	// DueDate format is "2006-01-02"
	assert.Contains(t, output, now.Format("2006-01-02"))
}

func TestFlashcardsList_Empty(t *testing.T) {
	// 1. Setup Temp Dir and Store
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	storePath := filepath.Join(tmpDir, ".recac", "flashcards.json")
	os.MkdirAll(filepath.Dir(storePath), 0755)

	// No cards
	data, _ := json.Marshal([]flashcards.Flashcard{})
	os.WriteFile(storePath, data, 0644)

	// 2. Run Command
	cmd := &cobra.Command{Use: "list", RunE: runFlashcardsList}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runFlashcardsList(cmd, []string{})
	assert.NoError(t, err)

	// 3. Verify Output
	output := buf.String()
	assert.Contains(t, output, "No cards found")
}

func TestRunFlashcardsStudy(t *testing.T) {
	// Mock Store
	originalGetStore := getStoreFunc
	defer func() { getStoreFunc = originalGetStore }()

	getStoreFunc = func() (flashcards.Store, error) {
		return &flashcards.FileStore{}, nil // Dummy store
	}

	// Mock Program Runner
	originalRunProgram := runFlashcardsProgramFunc
	defer func() { runFlashcardsProgramFunc = originalRunProgram }()

	called := false
	runFlashcardsProgramFunc = func(store flashcards.Store) error {
		called = true
		return nil
	}

	// Run Command
	cmd := &cobra.Command{Use: "study", RunE: runFlashcardsStudy}
	err := runFlashcardsStudy(cmd, []string{})

	assert.NoError(t, err)
	assert.True(t, called, "Expected flashcards study program to run")
}
