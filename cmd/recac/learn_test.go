package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/learn"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgent is already defined in tickets_test.go in the same package (main)

func TestLearnStats(t *testing.T) {
	// Setup temporary deck
	tmpDir := t.TempDir()

	// Create .recac directory in tmpDir
	recacDir := filepath.Join(tmpDir, ".recac")
	err := os.MkdirAll(recacDir, 0755)
	assert.NoError(t, err)

	// Switch CWD to tmpDir so GetDeckPath uses it
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Create Deck
	deck := learn.Deck{
		Cards: []learn.Card{
			{ID: "1", Interval: 1, NextReview: time.Now().Add(-1 * time.Hour)}, // Due
			{ID: "2", Interval: 25, NextReview: time.Now().Add(24 * time.Hour)}, // Mastered
			{ID: "3", Interval: 5, NextReview: time.Now().Add(24 * time.Hour)}, // Learning
		},
	}
	deckPath := filepath.Join(recacDir, "learn.json")
	data, _ := json.Marshal(deck)
	err = os.WriteFile(deckPath, data, 0644)
	assert.NoError(t, err)

	// Execute Stats
	buf := new(bytes.Buffer)
	learnStatsCmd.SetOut(buf)

	// We call the run function directly or via Execute()
	// Since we are in main package tests, we can call runLearnStats
	err = runLearnStats(learnStatsCmd, []string{})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Total Cards: 3")
	assert.Contains(t, output, "Due Now:     1")
	assert.Contains(t, output, "Mastered:    1")
	assert.Contains(t, output, "Learning:    2")
}

func TestLearnGenerate(t *testing.T) {
	// Setup temporary workspace
	tmpDir := t.TempDir()

	// Create dummy go file
	codePath := filepath.Join(tmpDir, "main.go")
	os.WriteFile(codePath, []byte("package main\nfunc main() {}"), 0644)

	// Create .recac dir
	recacDir := filepath.Join(tmpDir, ".recac")
	os.MkdirAll(recacDir, 0755)

	// Switch CWD
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Mock Agent
	mockAgent := new(MockAgent)
	mockResponse := `[{"question": "Q1", "answer": "A1"}]`
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(mockResponse, nil)

	// Override Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute Generate
	buf := new(bytes.Buffer)
	learnGenerateCmd.SetOut(buf)

	// We pass tmpDir as argument
	err := runLearnGenerate(learnGenerateCmd, []string{tmpDir})
	assert.NoError(t, err)

	// Verify Output
	assert.Contains(t, buf.String(), "Generated 1 new cards")

	// Verify Deck content
	deck, err := learn.LoadDeck()
	assert.NoError(t, err)
	assert.Len(t, deck.Cards, 1)
	assert.Equal(t, "Q1", deck.Cards[0].Question)
}
