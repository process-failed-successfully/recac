package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/agent"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
)

func TestUpdateCard(t *testing.T) {
	card := &Flashcard{
		Interval:    1,
		Repetitions: 1,
		Easiness:    2.5,
	}

	// Grade 5 (Perfect)
	// If reps=1, interval should become 6
	updateCard(card, 5)
	assert.Equal(t, 6, card.Interval)
	assert.Equal(t, 2, card.Repetitions)
	assert.True(t, card.Easiness > 2.5)

	// Reset
	card.Repetitions = 0
	card.Interval = 0
	card.Easiness = 2.5

	// Grade 3 (Pass)
	updateCard(card, 3)
	assert.Equal(t, 1, card.Interval) // Reps 0 -> 1
	assert.Equal(t, 1, card.Repetitions)
	assert.True(t, card.Easiness < 2.5) // 3 drops easiness

	// Grade 0 (Fail)
	updateCard(card, 0)
	assert.Equal(t, 1, card.Interval)
	assert.Equal(t, 0, card.Repetitions)
}

func TestLearnCmd_Generate(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-learn-gen-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	os.WriteFile("dummy.go", []byte("package main"), 0644)

	// 2. Mock Agent
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		mock := agent.NewMockAgent()
		mock.SetResponse(`{"question": "Q1", "answer": "A1"}`)
		return mock, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// 3. Mock Survey to Confirm Generation
	originalAskOne := askOneFunc
	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		if confirm, ok := p.(*survey.Confirm); ok {
			if confirm.Message == "Would you like to generate new flashcards from the codebase?" {
				*(response.(*bool)) = true
				return nil
			}
		}
		return nil
	}
	defer func() { askOneFunc = originalAskOne }()

	// 4. Run
	cmd := learnCmd
	// Set Out to discard
	// cmd.SetOut(io.Discard) // learnCmd logic uses cmd.OutOrStdout()
	err = runLearn(cmd, []string{})
	assert.NoError(t, err)

	// 5. Verify Saved
	cardsPath := filepath.Join(tmpDir, ".recac", "flashcards.json")
	assert.FileExists(t, cardsPath)
	b, _ := os.ReadFile(cardsPath)
	var cards []Flashcard
	json.Unmarshal(b, &cards)
	assert.Len(t, cards, 1)
	assert.Equal(t, "Q1", cards[0].Question)
}

func TestLearnCmd_Review(t *testing.T) {
	// 1. Setup Temp Dir with existing due card
	tmpDir, err := os.MkdirTemp("", "recac-learn-review-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Create due card
	card := Flashcard{
		ID:          "1",
		Question:    "Q1",
		Answer:      "A1",
		NextReview:  time.Now().Add(-24 * time.Hour), // Due yesterday
		Interval:    1,
		Repetitions: 1,
		Easiness:    2.5,
	}

	dir := filepath.Join(tmpDir, ".recac")
	os.MkdirAll(dir, 0755)
	b, _ := json.Marshal([]Flashcard{card})
	os.WriteFile(filepath.Join(dir, "flashcards.json"), b, 0644)

	// 2. Mock Survey (Review flow)
	originalAskOne := askOneFunc
	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		if _, ok := p.(*survey.Select); ok {
			// Rating prompt
			*(response.(*string)) = "5 - Bright (Instant recall)"
			return nil
		}
		if _, ok := p.(*survey.Confirm); ok {
			// Generate more? No.
			*(response.(*bool)) = false
			return nil
		}
		return nil
	}
	defer func() { askOneFunc = originalAskOne }()

	// 3. Mock Stdin for "Press Enter"
	r, w, _ := os.Pipe()
	originalStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = originalStdin }()

	go func() {
		w.Write([]byte("\n")) // Enter for review
		w.Close()
	}()

	// 4. Run
	cmd := learnCmd
	err = runLearn(cmd, []string{})
	assert.NoError(t, err)

	// 5. Verify Update
	b, _ = os.ReadFile(filepath.Join(dir, "flashcards.json"))
	var cards []Flashcard
	json.Unmarshal(b, &cards)

	assert.Len(t, cards, 1)
	assert.Equal(t, 2, cards[0].Repetitions) // 1 -> 2
	assert.Equal(t, 6, cards[0].Interval)    // 1 -> 6 (SM-2 rule for 2nd rep)
}
