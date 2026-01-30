package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/learn"
	"recac/internal/ui"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	learnLimit int
)

var learnCmd = &cobra.Command{
	Use:   "learn",
	Short: "Master the codebase with Spaced Repetition",
	Long:  `Codebase Mastery System.
Uses the SuperMemo-2 algorithm to schedule review of AI-generated flashcards about your code.`,
	RunE: runLearnStudy,
}

var learnGenCmd = &cobra.Command{
	Use:   "generate [path]",
	Short: "Generate flashcards from code",
	RunE:  runLearnGenerate,
}

var learnStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show learning progress",
	RunE:  runLearnStats,
}

func init() {
	rootCmd.AddCommand(learnCmd)
	learnCmd.AddCommand(learnGenCmd)
	learnCmd.AddCommand(learnStatsCmd)

	learnGenCmd.Flags().IntVarP(&learnLimit, "limit", "l", 5, "Maximum number of cards to generate")
}

func runLearnGenerate(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	ctx := context.Background()
	cwd, _ := os.Getwd()
	store := learn.NewStore(cwd)

	// Load Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-learn")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🧠 Generating up to %d flashcards from %s...\n", learnLimit, path)
	newCards, err := learn.GenerateCards(ctx, ag, path, learnLimit)
	if err != nil {
		return err
	}

	if len(newCards) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No cards generated.")
		return nil
	}

	// Save
	existingCards, _ := store.Load()

	// Dedup by ID or just append? IDs are somewhat unique (timestamp).
	existingCards = append(existingCards, newCards...)

	if err := store.Save(existingCards); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Added %d new cards. Total: %d\n", len(newCards), len(existingCards))
	return nil
}

func runLearnStudy(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	store := learn.NewStore(cwd)

	cards, err := store.Load()
	if err != nil {
		return fmt.Errorf("failed to load cards: %w", err)
	}

	if len(cards) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No flashcards found. Run 'recac learn generate' first.")
		return nil
	}

	// Filter due cards
	var dueCards []learn.Flashcard
	now := time.Now()
	for _, c := range cards {
		if c.NextReview.Before(now) {
			dueCards = append(dueCards, c)
		}
	}

	if len(dueCards) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "🎉 No cards due for review! Check back later.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "📚 Starting study session with %d cards...\n", len(dueCards))

	// Start TUI
	p := tea.NewProgram(ui.NewLearnModel(dueCards), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return err
	}

	finalModel, ok := m.(ui.LearnModel)
	if !ok {
		return fmt.Errorf("failed to get model")
	}

	// Update store with results
	if len(finalModel.UpdatedCards) > 0 {
		// Update map
		updatedMap := make(map[string]learn.Flashcard)
		for _, c := range finalModel.UpdatedCards {
			updatedMap[c.ID] = c
		}

		for i, c := range cards {
			if updated, exists := updatedMap[c.ID]; exists {
				cards[i] = updated
			}
		}

		if err := store.Save(cards); err != nil {
			return fmt.Errorf("failed to save progress: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "💾 Progress saved. Reviewed %d cards.\n", len(finalModel.UpdatedCards))
	}

	return nil
}

func runLearnStats(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	store := learn.NewStore(cwd)

	cards, err := store.Load()
	if err != nil {
		// If not exists, just say 0
		cards = []learn.Flashcard{}
	}

	total := len(cards)
	if total == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No flashcards generated yet.")
		return nil
	}

	learned := 0
	mastered := 0 // Interval > 21 days

	for _, c := range cards {
		if c.Repetitions > 0 {
			learned++
		}
		if c.Interval > 21 {
			mastered++
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Total Cards: %d\n", total)
	fmt.Fprintf(cmd.OutOrStdout(), "Learned:     %d (%.1f%%)\n", learned, float64(learned)/float64(total)*100)
	fmt.Fprintf(cmd.OutOrStdout(), "Mastered:    %d\n", mastered)

	return nil
}
