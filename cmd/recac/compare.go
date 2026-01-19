package main

import (
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var analyzeComparison bool

var compareCmd = &cobra.Command{
	Use:   "compare [session-a] [session-b]",
	Short: "Compare two sessions",
	Long: `Compare metrics and outcomes of two sessions.
Useful for A/B testing different models, prompts, or agents.
Displays a side-by-side comparison of duration, cost, tokens, and status.

Use --analyze to have the AI agent read the logs and provide a qualitative comparison of the approach.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionAName := args[0]
		sessionBName := args[1]

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		// Load Sessions
		sessionA, err := sm.LoadSession(sessionAName)
		if err != nil {
			return fmt.Errorf("failed to load session A (%s): %w", sessionAName, err)
		}

		sessionB, err := sm.LoadSession(sessionBName)
		if err != nil {
			return fmt.Errorf("failed to load session B (%s): %w", sessionBName, err)
		}

		// Load Agent States (Metrics)
		stateA := loadStateSafe(sessionA.AgentStateFile)
		stateB := loadStateSafe(sessionB.AgentStateFile)

		// Display Comparison Table
		displayComparison(cmd, sessionA, sessionB, stateA, stateB)

		// AI Analysis
		if analyzeComparison {
			if err := analyzeSessions(cmd, sm, sessionA, sessionB); err != nil {
				return fmt.Errorf("analysis failed: %w", err)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(compareCmd)
	compareCmd.Flags().BoolVar(&analyzeComparison, "analyze", false, "Use AI to analyze and compare the session logs")
}

func loadStateSafe(path string) *agent.State {
	if path == "" {
		return &agent.State{}
	}
	state, err := loadAgentState(path)
	if err != nil {
		// Return empty state if load fails (file might not exist yet)
		return &agent.State{}
	}
	return state
}

func displayComparison(cmd *cobra.Command, sA, sB *runner.SessionState, stA, stB *agent.State) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)

	fmt.Fprintf(w, "METRIC\tSESSION A (%s)\tSESSION B (%s)\n", sA.Name, sB.Name)
	fmt.Fprintf(w, "------\t--------------\t--------------\n")

	// Status
	fmt.Fprintf(w, "Status\t%s\t%s\n", sA.Status, sB.Status)

	// Goal
	goalA := truncate(sA.Goal, 30)
	goalB := truncate(sB.Goal, 30)
	fmt.Fprintf(w, "Goal\t%s\t%s\n", goalA, goalB)

	// Duration
	durA := calculateDuration(sA)
	durB := calculateDuration(sB)
	fmt.Fprintf(w, "Duration\t%s\t%s\n", durA, durB)

	// Model
	modelA := stA.Model
	if modelA == "" {
		modelA = "N/A"
	}
	modelB := stB.Model
	if modelB == "" {
		modelB = "N/A"
	}
	fmt.Fprintf(w, "Model\t%s\t%s\n", modelA, modelB)

	// Tokens
	fmt.Fprintf(w, "Tokens (Total)\t%d\t%d\n", stA.TokenUsage.TotalTokens, stB.TokenUsage.TotalTokens)
	fmt.Fprintf(w, "Tokens (Prompt)\t%d\t%d\n", stA.TokenUsage.TotalPromptTokens, stB.TokenUsage.TotalPromptTokens)
	fmt.Fprintf(w, "Tokens (Resp)\t%d\t%d\n", stA.TokenUsage.TotalResponseTokens, stB.TokenUsage.TotalResponseTokens)

	// Cost
	costA := agent.CalculateCost(stA.Model, stA.TokenUsage)
	costB := agent.CalculateCost(stB.Model, stB.TokenUsage)
	fmt.Fprintf(w, "Est. Cost\t$%.4f\t$%.4f\n", costA, costB)

	// Commit Difference
	filesA := "N/A"
	filesB := "N/A"

	// We can't easily get file count without running git diff, but we can check if EndCommitSHA is set
	if len(sA.EndCommitSHA) >= 7 {
		filesA = sA.EndCommitSHA[:7]
	} else if sA.EndCommitSHA != "" {
		filesA = sA.EndCommitSHA
	}

	if len(sB.EndCommitSHA) >= 7 {
		filesB = sB.EndCommitSHA[:7]
	} else if sB.EndCommitSHA != "" {
		filesB = sB.EndCommitSHA
	}
	fmt.Fprintf(w, "End Commit\t%s\t%s\n", filesA, filesB)

	w.Flush()
}

func calculateDuration(s *runner.SessionState) time.Duration {
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime).Round(time.Second)
	}
	return s.EndTime.Sub(s.StartTime).Round(time.Second)
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func analyzeSessions(cmd *cobra.Command, sm ISessionManager, sA, sB *runner.SessionState) error {
	fmt.Fprintln(cmd.OutOrStdout(), "\n🤖 Analyzing logs with AI... (this may take a moment)")

	logsA, err := sm.GetSessionLogContent(sA.Name, 100) // Last 100 lines
	if err != nil {
		logsA = "Could not read logs."
	}
	logsB, err := sm.GetSessionLogContent(sB.Name, 100)
	if err != nil {
		logsB = "Could not read logs."
	}

	prompt := fmt.Sprintf(`Compare these two coding sessions and determine which one was more effective and why.
Focus on the approach, errors encountered, and final outcome.

SESSION A (%s):
Goal: %s
Logs (Snippet):
'''
%s
'''

SESSION B (%s):
Goal: %s
Logs (Snippet):
'''
%s
'''

Provide a concise comparison summary.`,
		sA.Name, sA.Goal, logsA,
		sB.Name, sB.Goal, logsB)

	ctx := cmd.Context()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-compare")
	if err != nil {
		return err
	}

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\n--- AI Analysis ---")
	fmt.Fprintln(cmd.OutOrStdout(), resp)
	return nil
}
