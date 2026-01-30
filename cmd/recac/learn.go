package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"recac/internal/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Flashcard represents a spaced-repetition card
type Flashcard struct {
	ID          string   `json:"id"`
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	Correct     string   `json:"correct_answer"`
	Explanation string   `json:"explanation"`

	// SRS Fields
	ReviewCount  int       `json:"review_count"`
	LastReviewed time.Time `json:"last_reviewed"`
	NextReview   time.Time `json:"next_review"`
	Interval     float64   `json:"interval"` // In days
	EaseFactor   float64   `json:"ease_factor"`
}

var (
	learnAddCount int
)

var learnCmd = &cobra.Command{
	Use:   "learn",
	Short: "Master the codebase using Spaced Repetition",
	Long: `Learn helps you master the codebase by generating flashcards from the code
and scheduling reviews using the SM-2 spaced repetition algorithm.`,
	RunE: runLearnReview,
}

var learnAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Generate and add new flashcards",
	RunE:  runLearnAdd,
}

var learnStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show learning statistics",
	RunE:  runLearnStats,
}

func init() {
	rootCmd.AddCommand(learnCmd)
	learnCmd.AddCommand(learnAddCmd)
	learnCmd.AddCommand(learnStatsCmd)

	learnAddCmd.Flags().IntVarP(&learnAddCount, "count", "n", 3, "Number of cards to add")
}

func getFlashcardsPath() string {
	home, _ := os.UserHomeDir()
	// Default to local project .recac folder if possible, else global
	cwd, _ := os.Getwd()
	localDir := filepath.Join(cwd, ".recac")
	if _, err := os.Stat(localDir); err == nil {
		return filepath.Join(localDir, "flashcards.json")
	}
	// Fallback to home
	return filepath.Join(home, ".recac", "flashcards.json")
}

func loadFlashcards() ([]Flashcard, error) {
	path := getFlashcardsPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []Flashcard{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cards []Flashcard
	if err := json.Unmarshal(data, &cards); err != nil {
		return nil, err
	}
	return cards, nil
}

func saveFlashcards(cards []Flashcard) error {
	path := getFlashcardsPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cards, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func runLearnAdd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, _ := os.Getwd()

	cards, err := loadFlashcards()
	if err != nil {
		return fmt.Errorf("failed to load cards: %w", err)
	}

	fmt.Printf("Generating %d new flashcards...\n", learnAddCount)

	// Initialize Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-learn")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	for i := 0; i < learnAddCount; i++ {
		fmt.Printf("[%d/%d] Finding file and generating question...\n", i+1, learnAddCount)

		// Reuse getRandomGoFile from quiz.go
		content, path, err := getRandomGoFile(cwd)
		if err != nil {
			fmt.Printf("Failed to get random file: %v\n", err)
			continue
		}

		prompt := fmt.Sprintf(`Create a challenging multiple-choice question based on the following Go code.
Code from %s:
%s

Return the result as a raw JSON object with the following structure:
{
    "question": "The question text",
    "options": ["Option A", "Option B", "Option C", "Option D"],
    "correct_answer": "The correct option text (must be exact match)",
    "explanation": "Explanation of why it is correct"
}
Do not use markdown blocks.`, path, content)

		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			fmt.Printf("Agent failed: %v\n", err)
			continue
		}

		jsonStr := utils.CleanJSONBlock(resp)
		var q struct {
			Question    string   `json:"question"`
			Options     []string `json:"options"`
			Correct     string   `json:"correct_answer"`
			Explanation string   `json:"explanation"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &q); err != nil {
			fmt.Printf("Failed to parse JSON: %v\n", err)
			continue
		}

		newCard := Flashcard{
			ID:           uuid.New().String(),
			Question:     q.Question,
			Options:      q.Options,
			Correct:      q.Correct,
			Explanation:  q.Explanation,
			ReviewCount:  0,
			EaseFactor:   2.5,
			Interval:     0,
			LastReviewed: time.Time{}, // Zero time
			NextReview:   time.Now(),  // Due immediately
		}

		cards = append(cards, newCard)
		fmt.Printf("✅ Added card for %s\n", path)
	}

	if err := saveFlashcards(cards); err != nil {
		return fmt.Errorf("failed to save cards: %w", err)
	}

	fmt.Printf("\nDone! You now have %d flashcards.\nRun 'recac learn' to review them.\n", len(cards))
	return nil
}

func runLearnReview(cmd *cobra.Command, args []string) error {
	cards, err := loadFlashcards()
	if err != nil {
		return fmt.Errorf("failed to load cards: %w", err)
	}

	now := time.Now()
	var dueCards []*Flashcard
	for i := range cards {
		if cards[i].NextReview.Before(now) {
			dueCards = append(dueCards, &cards[i])
		}
	}

	if len(dueCards) == 0 {
		fmt.Println("🎉 No cards due for review! Great job.")
		fmt.Println("Use 'recac learn add' to create more cards.")
		return nil
	}

	fmt.Printf("📝 Reviewing %d cards...\n\n", len(dueCards))

	for i, card := range dueCards {
		fmt.Printf("--- Card %d/%d ---\n", i+1, len(dueCards))

		var answer string
		prompt := &survey.Select{
			Message: card.Question,
			Options: card.Options,
		}
		if err := survey.AskOne(prompt, &answer); err != nil {
			return err
		}

		isCorrect := (answer == card.Correct)
		if isCorrect {
			fmt.Println("\n✅ Correct!")
		} else {
			fmt.Printf("\n❌ Incorrect. The answer was: %s\n", card.Correct)
		}
		fmt.Printf("💡 %s\n\n", card.Explanation)

		// Ask for Grade
		var gradeStr string
		gradeOptions := []string{"Again (Fail)", "Hard", "Good", "Easy"}
		gradePrompt := &survey.Select{
			Message: "How was this card?",
			Options: gradeOptions,
		}
		if err := survey.AskOne(gradePrompt, &gradeStr); err != nil {
			return err
		}

		grade := 0
		switch gradeStr {
		case "Again (Fail)":
			grade = 0
		case "Hard":
			grade = 3
		case "Good":
			grade = 4
		case "Easy":
			grade = 5
		}

		updateCardSRS(card, grade)
	}

	if err := saveFlashcards(cards); err != nil {
		return fmt.Errorf("failed to save progress: %w", err)
	}

	fmt.Println("\n✅ Session complete!")
	return nil
}

func runLearnStats(cmd *cobra.Command, args []string) error {
	cards, err := loadFlashcards()
	if err != nil {
		return fmt.Errorf("failed to load cards: %w", err)
	}

	total := len(cards)
	learned := 0
	due := 0
	now := time.Now()

	for _, c := range cards {
		if c.Interval > 21 { // Arbitrary "learned" threshold > 3 weeks
			learned++
		}
		if c.NextReview.Before(now) {
			due++
		}
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Total Cards:\t%d\n", total)
	fmt.Fprintf(w, "Due Now:\t%d\n", due)
	fmt.Fprintf(w, "Mastered (>21d):\t%d\n", learned)
	w.Flush()

	return nil
}

// updateCardSRS implements the SM-2 algorithm
func updateCardSRS(card *Flashcard, grade int) {
	if grade < 3 {
		card.ReviewCount = 0
		card.Interval = 1
		card.NextReview = time.Now().Add(24 * time.Hour)
		return
	}

	// Calculate New Ease Factor
	// EF' = EF + (0.1 - (5-q)*(0.08 + (5-q)*0.02))
	newEase := card.EaseFactor + (0.1 - float64(5-grade)*(0.08+float64(5-grade)*0.02))
	if newEase < 1.3 {
		newEase = 1.3
	}
	card.EaseFactor = newEase

	// Calculate Interval
	if card.ReviewCount == 0 {
		card.Interval = 1
	} else if card.ReviewCount == 1 {
		card.Interval = 6
	} else {
		card.Interval = math.Ceil(card.Interval * card.EaseFactor)
	}

	card.ReviewCount++
	card.LastReviewed = time.Now()
	card.NextReview = time.Now().Add(time.Duration(card.Interval*24) * time.Hour)
}
