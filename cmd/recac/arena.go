package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type AgentFactory func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error)

func NewArenaCmd(factory AgentFactory) *cobra.Command {
	var (
		arenaCompetitors string
		arenaTask        string
		arenaFile        string
		arenaJudgeProv   string
		arenaJudgeModel  string
	)

	cmd := &cobra.Command{
		Use:   "arena",
		Short: "Pit multiple AI models against each other",
		Long: `Run the same task against multiple AI models in parallel and use a Judge Agent to evaluate the results.

Example:
  recac arena --competitors "openai:gpt-4,gemini:gemini-pro" --task "Explain quantum computing"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			competitorList := strings.Split(arenaCompetitors, ",")
			// Trim spaces
			for i := range competitorList {
				competitorList[i] = strings.TrimSpace(competitorList[i])
			}

			// Filter empty
			var cleanCompetitors []string
			for _, c := range competitorList {
				if c != "" {
					cleanCompetitors = append(cleanCompetitors, c)
				}
			}

			if len(cleanCompetitors) < 2 {
				return fmt.Errorf("arena requires at least 2 competitors")
			}

			ctx := cmd.Context()
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			// Read context file
			var fileContext string
			if arenaFile != "" {
				content, err := os.ReadFile(arenaFile)
				if err != nil {
					return fmt.Errorf("failed to read context file: %w", err)
				}
				fileContext = string(content)
			}

			// Prepare Prompt
			fullPrompt := arenaTask
			if fileContext != "" {
				fullPrompt += fmt.Sprintf("\n\nContext:\n```\n%s\n```", fileContext)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "🏟️  Starting Arena with %d competitors...\n", len(cleanCompetitors))

			results := make([]ArenaResult, len(cleanCompetitors))
			var wg sync.WaitGroup
			var outMu sync.Mutex

			for i, compStr := range cleanCompetitors {
				wg.Add(1)
				go func(index int, cStr string) {
					defer wg.Done()

					parts := strings.SplitN(cStr, ":", 2)
					provider := parts[0]
					model := ""
					if len(parts) > 1 {
						model = parts[1]
					}

					// Sanity check
					if provider == "" {
						results[index] = ArenaResult{Error: fmt.Errorf("invalid competitor string: %s", cStr)}
						return
					}

					start := time.Now()

					projName := fmt.Sprintf("recac-arena-%d", index)

					ag, err := factory(ctx, provider, model, cwd, projName)
					if err != nil {
						results[index] = ArenaResult{Provider: provider, Model: model, Error: err, Duration: time.Since(start)}
						return
					}

					resp, err := ag.Send(ctx, fullPrompt)
					duration := time.Since(start)

					results[index] = ArenaResult{
						Provider: provider,
						Model:    model,
						Response: resp,
						Duration: duration,
						Error:    err,
					}

					outMu.Lock()
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "❌ %s:%s failed: %v\n", provider, model, err)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "✅ %s:%s finished in %v\n", provider, model, duration)
					}
					outMu.Unlock()

				}(i, compStr)
			}

			wg.Wait()

			// Filter valid results
			var validResults []ArenaResult
			for _, res := range results {
				if res.Error == nil {
					validResults = append(validResults, res)
				}
			}

			if len(validResults) == 0 {
				return fmt.Errorf("all competitors failed")
			}

			if len(validResults) == 1 {
				fmt.Fprintln(cmd.OutOrStdout(), "Only one competitor succeeded. No judging possible.")
				return nil
			}

			// Judging Phase
			judgeProv := arenaJudgeProv
			if judgeProv == "" {
				judgeProv = viper.GetString("provider")
			}
			judgeMod := arenaJudgeModel
			if judgeMod == "" {
				judgeMod = viper.GetString("model")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n⚖️  Judging with %s:%s...\n", judgeProv, judgeMod)

			judgeAgent, err := factory(ctx, judgeProv, judgeMod, cwd, "recac-arena-judge")
			if err != nil {
				return fmt.Errorf("failed to create judge agent: %w", err)
			}

			var judgePromptBuilder strings.Builder
			judgePromptBuilder.WriteString("You are an impartial Judge. Evaluate the following responses to the task.\n")
			judgePromptBuilder.WriteString(fmt.Sprintf("Task: %s\n\n", arenaTask))

			for i, res := range validResults {
				judgePromptBuilder.WriteString(fmt.Sprintf("--- Candidate %d ---\n", i+1))
				judgePromptBuilder.WriteString(res.Response)
				judgePromptBuilder.WriteString("\n\n")
			}

			judgePromptBuilder.WriteString(`
Evaluate the responses based on correctness, clarity, and completeness.
Decide which candidate is the winner.
Return your decision in the following format:
WINNER: Candidate X
REASONING: [Your explanation]
`)

			judgeResp, err := judgeAgent.Send(ctx, judgePromptBuilder.String())
			if err != nil {
				return fmt.Errorf("judge failed: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "\n🏆 Arena Results 🏆")
			fmt.Fprintln(cmd.OutOrStdout(), "===================")
			fmt.Fprintln(cmd.OutOrStdout(), judgeResp)

			// Print Key
			fmt.Fprintln(cmd.OutOrStdout(), "\nKey:")
			for i, res := range validResults {
				fmt.Fprintf(cmd.OutOrStdout(), "Candidate %d: %s:%s (Time: %v)\n", i+1, res.Provider, res.Model, res.Duration)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&arenaCompetitors, "competitors", "c", "", "Comma-separated list of provider:model pairs (e.g., openai:gpt-4,gemini:gemini-pro)")
	cmd.Flags().StringVarP(&arenaTask, "task", "t", "", "The task or question to evaluate")
	cmd.Flags().StringVarP(&arenaFile, "file", "f", "", "Optional file to include as context")
	cmd.Flags().StringVar(&arenaJudgeProv, "judge-provider", "", "Provider for the judge (default: config)")
	cmd.Flags().StringVar(&arenaJudgeModel, "judge-model", "", "Model for the judge (default: config)")

	cmd.MarkFlagRequired("competitors")
	cmd.MarkFlagRequired("task")

	return cmd
}

func initArenaCmd(rootCmd *cobra.Command) {
	arenaCmd := NewArenaCmd(agentClientFactory)
	rootCmd.AddCommand(arenaCmd)
}

type ArenaResult struct {
	Provider string
	Model    string
	Response string
	Duration time.Duration
	Error    error
}
