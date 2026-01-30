package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"recac/internal/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Flashcard represents a single item in the spaced repetition system
type Flashcard struct {
	ID          string    `json:"id"`
	Question    string    `json:"question"`
	Answer      string    `json:"answer"` // Or "Back" of the card
	FilePath    string    `json:"file_path,omitempty"`
	Interval    int       `json:"interval"`     // Days until next review
	Repetitions int       `json:"repetitions"`  // Number of times reviewed
	Easiness    float64   `json:"easiness"`     // SM-2 Easiness Factor (default 2.5)
	NextReview  time.Time `json:"next_review"`  // When this card is due
	Created     time.Time `json:"created_at"`
}

var learnCmd = &cobra.Command{
	Use:   "learn",
	Short: "Master the codebase using Spaced Repetition (SM-2)",
	Long: `Learn the codebase using a Spaced Repetition System (SRS).
This command manages a deck of flashcards generated from your code.
Cards are scheduled for review based on your performance (SM-2 algorithm).
Run this command daily to keep your knowledge fresh!`,
	RunE: runLearn,
}

func init() {
	rootCmd.AddCommand(learnCmd)
}

func runLearn(cmd *cobra.Command, args []string) error {
	// 1. Load Flashcards
	cards, err := loadFlashcards()
	if err != nil {
		return err
	}

	// 2. Identify Due Cards
	var due []*Flashcard
	now := time.Now()
	for i := range cards {
		if cards[i].NextReview.Before(now) {
			due = append(due, &cards[i])
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "📚 You have %d cards due for review (Total deck: %d)\n", len(due), len(cards))

	// 3. Review Loop
	if len(due) > 0 {
		if err := reviewCards(cmd, due); err != nil {
			return err
		}
		// Save progress after review
		if err := saveFlashcards(cards); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "✅ Review complete! Progress saved.")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "🎉 You are all caught up!")
	}

	// 4. Offer to Generate New Cards
	// Only if user wants to learn more
	confirm := false
	prompt := &survey.Confirm{
		Message: "Would you like to generate new flashcards from the codebase?",
		Default: len(cards) == 0, // Default yes if deck is empty
	}
	// We handle error gracefully as user might just ctrl-c
	if err := askOneFunc(prompt, &confirm); err != nil {
		return nil
	}

	if confirm {
		newCards, err := generateFlashcards(cmd)
		if err != nil {
			return err
		}
		cards = append(cards, newCards...)
		if err := saveFlashcards(cards); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✨ Added %d new cards to your deck.\n", len(newCards))
	}

	return nil
}

func reviewCards(cmd *cobra.Command, cards []*Flashcard) error {
	for i, card := range cards {
		fmt.Fprintf(cmd.OutOrStdout(), "\n--- Card %d/%d ---\n", i+1, len(cards))
		if card.FilePath != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "File: %s\n", card.FilePath)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Q: %s\n", card.Question)

		// Wait for user to press Enter to show answer
		fmt.Fprintln(cmd.OutOrStdout(), "(Press Enter to show answer)")
		var ignore string
		fmt.Scanln(&ignore)

		fmt.Fprintf(cmd.OutOrStdout(), "A: %s\n", card.Answer)

		// Rate performance
		rating := 0
		prompt := &survey.Select{
			Message: "How well did you know this?",
			Options: []string{
				"0 - Blackout (Complete failure)",
				"1 - Incorrect (Wrong answer)",
				"2 - Hard (Correct but difficult)",
				"3 - Good (Correct with hesitation)",
				"4 - Easy (Perfect recall)",
				"5 - Bright (Instant recall)",
			},
			PageSize: 6,
		}
		var choice string
		if err := askOneFunc(prompt, &choice); err != nil {
			return err
		}

		// Parse rating from string "N - ..."
		fmt.Sscanf(choice, "%d", &rating)

		// Update Card (SM-2)
		updateCard(card, rating)
	}
	return nil
}

// updateCard applies the SM-2 algorithm
func updateCard(card *Flashcard, grade int) {
	if grade >= 3 {
		if card.Repetitions == 0 {
			card.Interval = 1
		} else if card.Repetitions == 1 {
			card.Interval = 6
		} else {
			card.Interval = int(math.Round(float64(card.Interval) * card.Easiness))
		}
		card.Repetitions++
	} else {
		card.Repetitions = 0
		card.Interval = 1
	}

	card.Easiness = card.Easiness + (0.1 - (5.0-float64(grade))*(0.08+(5.0-float64(grade))*0.02))
	if card.Easiness < 1.3 {
		card.Easiness = 1.3
	}

	card.NextReview = time.Now().AddDate(0, 0, card.Interval)
}

func generateFlashcards(cmd *cobra.Command) ([]Flashcard, error) {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	fileContent, filePath, err := getRandomGoFile(cwd)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "📄 Analyzing %s...\n", filePath)

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-learn")
	if err != nil {
		return nil, err
	}

	prompt := fmt.Sprintf(`You are a technical mentor.
Create a flashcard (Question and Answer) based on the following code to help a developer learn the codebase.
Focus on the "why" and "how" of the code, architectural decisions, or complex logic.
Avoid trivial syntax questions.

Code:
%s

Return the result as a raw JSON object:
{
    "question": "The question text",
    "answer": "The concise answer/explanation"
}
`, fileContent)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	jsonStr := utils.CleanJSONBlock(resp)
	var result struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse agent response: %w", err)
	}

	card := Flashcard{
		ID:          uuid.New().String(),
		Question:    result.Question,
		Answer:      result.Answer,
		FilePath:    filePath,
		Interval:    0,
		Repetitions: 0,
		Easiness:    2.5,
		NextReview:  time.Now(), // Due immediately
		Created:     time.Now(),
	}

	return []Flashcard{card}, nil
}

func getFlashcardsPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cwd, ".recac")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "flashcards.json"), nil
}

func loadFlashcards() ([]Flashcard, error) {
	path, err := getFlashcardsPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []Flashcard{}, nil
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cards []Flashcard
	if err := json.Unmarshal(b, &cards); err != nil {
		return nil, err
	}
	return cards, nil
}

func saveFlashcards(cards []Flashcard) error {
	path, err := getFlashcardsPath()
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(cards, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, b, 0644)
}
