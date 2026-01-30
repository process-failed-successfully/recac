package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/learn"
	"recac/internal/ui"
	"recac/internal/utils"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var learnCmd = &cobra.Command{
	Use:   "learn",
	Short: "Master the codebase using Spaced Repetition",
	Long: `An interactive learning tool that generates flashcards from your code
and uses the SM-2 algorithm to schedule reviews, helping you internalize the project structure and logic.`,
}

var learnGenerateCmd = &cobra.Command{
	Use:   "generate [path]",
	Short: "Generate flashcards from source code",
	RunE:  runLearnGenerate,
}

var learnReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Start a review session for due cards",
	RunE:  runLearnReview,
}

var learnStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show learning progress",
	RunE:  runLearnStats,
}

func init() {
	rootCmd.AddCommand(learnCmd)
	learnCmd.AddCommand(learnGenerateCmd)
	learnCmd.AddCommand(learnReviewCmd)
	learnCmd.AddCommand(learnStatsCmd)
}

func runLearnGenerate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	target := cwd
	if len(args) > 0 {
		target = args[0]
	}

	// 1. Find Files
	files, err := collectLearnableFiles(target)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no suitable files found in %s", target)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d files. Generating cards for a subset...\n", len(files))

	// Limit to 3 files for generation batch
	limit := 3
	if len(files) > limit {
		files = files[:limit]
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-learn")
	if err != nil {
		return err
	}

	deck, err := learn.LoadDeck()
	if err != nil {
		return err
	}

	newCardsCount := 0

	for _, file := range files {
		fmt.Fprintf(cmd.OutOrStdout(), "Analyzing %s...\n", file)
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Truncate content
		sContent := string(content)
		if len(sContent) > 3000 {
			sContent = sContent[:3000] + "\n...(truncated)"
		}

		prompt := fmt.Sprintf(`Analyze the following code and generate 1-3 flashcards to test understanding of its key concepts, architecture, or logic.
Focus on "Why" and "How", not just syntax.

Code (%s):
'''
%s
'''

Return ONLY a JSON array of objects:
[
  {
    "question": "What is the purpose of function X?",
    "answer": "It handles Y by doing Z."
  }
]`, file, sContent)

		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to generate for %s: %v\n", file, err)
			continue
		}

		jsonStr := utils.CleanJSONBlock(resp)

		type GenCard struct {
			Question string `json:"question"`
			Answer   string `json:"answer"`
		}
		var genCards []GenCard
		if err := json.Unmarshal([]byte(jsonStr), &genCards); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to parse JSON for %s: %v\n", file, err)
			continue
		}

		for _, gc := range genCards {
			card := learn.Card{
				ID:          uuid.New().String(),
				Question:    gc.Question,
				Answer:      gc.Answer,
				ContextFile: file,
				Created:     time.Now(),
				NextReview:  time.Now(), // Due immediately
				Interval:    0,
				EaseFactor:  2.5,
			}
			deck.Add(card)
			newCardsCount++
		}
	}

	if err := learn.SaveDeck(deck); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Generated %d new cards.\n", newCardsCount)
	return nil
}

func runLearnReview(cmd *cobra.Command, args []string) error {
	deck, err := learn.LoadDeck()
	if err != nil {
		return err
	}

	due := deck.GetDueCards()
	if len(due) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "🎉 No cards due for review! Good job.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Starting review for %d cards...\n", len(due))

	p := tea.NewProgram(ui.NewLearnModel(deck), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

func runLearnStats(cmd *cobra.Command, args []string) error {
	deck, err := learn.LoadDeck()
	if err != nil {
		return err
	}

	total := len(deck.Cards)
	due := len(deck.GetDueCards())
	mastered := 0 // Interval > 21 days
	learning := 0

	for _, c := range deck.Cards {
		if c.Interval > 21 {
			mastered++
		} else {
			learning++
		}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🧠 Learning Stats")
	fmt.Fprintln(cmd.OutOrStdout(), "----------------")
	fmt.Fprintf(cmd.OutOrStdout(), "Total Cards: %d\n", total)
	fmt.Fprintf(cmd.OutOrStdout(), "Due Now:     %d\n", due)
	fmt.Fprintf(cmd.OutOrStdout(), "Mastered:    %d (Interval > 21 days)\n", mastered)
	fmt.Fprintf(cmd.OutOrStdout(), "Learning:    %d\n", learning)

	return nil
}

func collectLearnableFiles(root string) ([]string, error) {
	var files []string
	ignore := DefaultIgnoreMap()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if ignore[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
