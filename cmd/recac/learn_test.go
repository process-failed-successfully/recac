package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock Agent
type TestMockAgent struct {
	Response string
}

func (m *TestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}
func (m *TestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestUpdateCardSRS(t *testing.T) {
	// Case 1: New card, good answer
	card := &Flashcard{
		EaseFactor:  2.5,
		ReviewCount: 0,
		Interval:    0,
	}
	updateCardSRS(card, 4) // Good (Grade 4)
	// EF' = 2.5 + (0.1 - (5-4)*(0.08 + (5-4)*0.02)) = 2.5 + (0.1 - 1*0.1) = 2.5
	assert.Equal(t, 1, card.ReviewCount)
	assert.Equal(t, 1.0, card.Interval)
	assert.Equal(t, 2.5, card.EaseFactor)

	// Case 2: Second review, good answer
	updateCardSRS(card, 4)
	assert.Equal(t, 2, card.ReviewCount)
	assert.Equal(t, 6.0, card.Interval)

	// Case 3: Third review, easy answer
	// EF' = 2.5 + (0.1 - (5-5)*...) = 2.6
	updateCardSRS(card, 5) // Easy (Grade 5)
	assert.Equal(t, 3, card.ReviewCount)
	// Interval = ceil(6 * 2.6) = 16
	assert.Equal(t, 16.0, card.Interval)
	assert.InDelta(t, 2.6, card.EaseFactor, 0.001)

	// Case 4: Fail
	updateCardSRS(card, 0)
	assert.Equal(t, 0, card.ReviewCount)
	assert.Equal(t, 1.0, card.Interval)
}

func TestRunLearnAdd(t *testing.T) {
	// Setup Temp Dir
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	// Create dummy go file
	err := os.WriteFile("dummy.go", []byte("package main\nfunc foo() {}"), 0644)
	require.NoError(t, err)

	// Create .recac dir
	err = os.Mkdir(".recac", 0755)
	require.NoError(t, err)

	// Mock Agent Factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &TestMockAgent{
			Response: `{"question": "What is foo?", "options": ["A", "B"], "correct_answer": "A", "explanation": "It is foo"}`,
		}, nil
	}

	// Run Command
	learnAddCount = 1
	cmd := learnAddCmd
	err = runLearnAdd(cmd, []string{})
	require.NoError(t, err)

	// Verify File
	cardsPath := filepath.Join(tmpDir, ".recac", "flashcards.json")
	require.FileExists(t, cardsPath)

	content, err := os.ReadFile(cardsPath)
	require.NoError(t, err)

	var cards []Flashcard
	err = json.Unmarshal(content, &cards)
	require.NoError(t, err)
	assert.Len(t, cards, 1)
	assert.Equal(t, "What is foo?", cards[0].Question)
	assert.Equal(t, "A", cards[0].Correct)
}
