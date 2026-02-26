package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/flashcards"
	"recac/internal/tui"
	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flashcardsTopic string
	flashcardsLimit int
	flashcardsFocus string
)

var flashcardsCmd = &cobra.Command{
	Use:   "flashcards",
	Short: "Spaced repetition learning for your codebase",
	Long: `Manage and study flashcards to master the codebase using the SuperMemo-2 algorithm.
Cards are stored in .recac/flashcards.json.`,
}

var flashcardsGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate new flashcards using AI",
	RunE:  runFlashcardsGenerate,
}

var flashcardsStudyCmd = &cobra.Command{
	Use:   "study",
	Short: "Start a study session for due cards",
	RunE:  runFlashcardsStudy,
}

var flashcardsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show learning statistics",
	RunE:  runFlashcardsStats,
}

var flashcardsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all flashcards",
	RunE:  runFlashcardsList,
}

func init() {
	rootCmd.AddCommand(flashcardsCmd)
	flashcardsCmd.AddCommand(flashcardsGenerateCmd)
	flashcardsCmd.AddCommand(flashcardsStudyCmd)
	flashcardsCmd.AddCommand(flashcardsStatsCmd)
	flashcardsCmd.AddCommand(flashcardsListCmd)

	flashcardsGenerateCmd.Flags().StringVarP(&flashcardsTopic, "topic", "t", "general", "Topic tag for new cards")
	flashcardsGenerateCmd.Flags().IntVarP(&flashcardsLimit, "limit", "l", 5, "Number of cards to generate")
	flashcardsGenerateCmd.Flags().StringVarP(&flashcardsFocus, "focus", "f", ".", "Directory to analyze")
}

func getStore() (flashcards.Store, error) {
	path, err := flashcards.DefaultStorePath()
	if err != nil {
		return nil, err
	}
	store := flashcards.NewFileStore(path)
	if err := store.Load(); err != nil {
		return nil, err
	}
	return store, nil
}

func runFlashcardsStudy(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}
	return tui.StartFlashcardsSession(store)
}

func runFlashcardsStats(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	cards := store.List()
	due := store.GetDue()

	total := len(cards)
	newCards := 0
	learning := 0
	review := 0
	relearning := 0

	for _, c := range cards {
		switch c.State {
		case flashcards.StateNew:
			newCards++
		case flashcards.StateLearning:
			learning++
		case flashcards.StateReview:
			review++
		case flashcards.StateRelearning:
			relearning++
		}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "📚 Flashcards Statistics")
	fmt.Fprintln(cmd.OutOrStdout(), "-----------------------")
	fmt.Fprintf(cmd.OutOrStdout(), "Total Cards:   %d\n", total)
	fmt.Fprintf(cmd.OutOrStdout(), "Due Today:     %d\n", len(due))
	fmt.Fprintln(cmd.OutOrStdout(), "")
	fmt.Fprintf(cmd.OutOrStdout(), "New:           %d\n", newCards)
	fmt.Fprintf(cmd.OutOrStdout(), "Learning:      %d\n", learning)
	fmt.Fprintf(cmd.OutOrStdout(), "Review:        %d\n", review)
	fmt.Fprintf(cmd.OutOrStdout(), "Relearning:    %d\n", relearning)

	return nil
}

func runFlashcardsList(cmd *cobra.Command, args []string) error {
	store, err := getStore()
	if err != nil {
		return err
	}

	cards := store.List()
	if len(cards) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No cards found.")
		return nil
	}

	for _, c := range cards {
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s (Next: %s)\n", c.Topic, c.Question, c.DueDate.Format("2006-01-02"))
	}
	return nil
}

func runFlashcardsGenerate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// 1. Context
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing codebase...")
	opts := ContextOptions{
		Roots:   []string{flashcardsFocus},
		MaxSize: 50 * 1024,
		Tree:    true,
	}
	// generateContextFunc is defined in factories.go (package main)
	codebaseContext, err := generateContextFunc(opts)
	if err != nil {
		return fmt.Errorf("failed to generate context: %w", err)
	}

	// 2. Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-flashcards")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// 3. Prompt
	prompt := fmt.Sprintf(`Analyze the provided codebase and generate %d flashcards for long-term memorization.
The goal is to help a new developer master the system's architecture, key data structures, and important functions.

Focus on:
- What does struct X represent?
- What is the responsibility of package Y?
- How does function Z work?
- Key interface definitions.

Return ONLY a JSON array of objects with keys: "question", "answer", "topic".
Topic should be related to the module or package name.
Tag all cards with topic '%s' if applicable, or specific sub-topics.

CONTEXT:
%s`, flashcardsLimit, flashcardsTopic, codebaseContext)

	fmt.Fprintln(cmd.OutOrStdout(), "🧠 Generating flashcards (thinking)...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 4. Parse
	type AIResponse struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
		Topic    string `json:"topic"`
	}

	cleanResp := utils.CleanJSONBlock(resp)
	var generated []AIResponse
	if err := json.Unmarshal([]byte(cleanResp), &generated); err != nil {
		return fmt.Errorf("failed to parse agent response: %w\n%s", err, resp)
	}

	// 5. Save
	store, err := getStore()
	if err != nil {
		return err
	}

	count := 0
	for _, g := range generated {
		topic := g.Topic
		if topic == "" {
			topic = flashcardsTopic
		}
		card := flashcards.NewFlashcard(g.Question, g.Answer, flashcardsFocus, topic)
		store.Add(card)
		count++
	}

	if err := store.Save(); err != nil {
		return fmt.Errorf("failed to save cards: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Generated and saved %d flashcards.\n", count)
	return nil
}
