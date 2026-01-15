package main

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"recac/internal/agent"

	"github.com/spf13/cobra"
)

// Summary holds the calculated statistics for the summary command.
type Summary struct {
	TotalSessions       int
	SuccessfulSessions  int
	FailedSessions      int
	RunningSessions     int
	SuccessRate         float64
	TotalTokens         int
	TotalPromptTokens   int
	TotalResponseTokens int
	TotalCost           float64
	Timeframe           string
}

func init() {
	rootCmd.AddCommand(summaryCmd)
	summaryCmd.Flags().String("since", "24h", "Summarize activity since a specific duration (e.g., '7d', '24h')")
}

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show a summary of session activity",
	Long:  `Displays a summary of session activity, including success rates, token usage, and costs within a given timeframe.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		since, _ := cmd.Flags().GetString("since")

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("could not create session manager: %w", err)
		}

		summary, err := calculateSummary(sm, since)
		if err != nil {
			return fmt.Errorf("could not calculate summary: %w", err)
		}

		displaySummary(cmd.OutOrStdout(), summary)
		return nil
	},
}

// calculateSummary computes the summary statistics for sessions within the given timeframe.
func calculateSummary(sm ISessionManager, since string) (*Summary, error) {
	sessions, err := sm.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("could not list sessions: %w", err)
	}

	sinceDuration, err := time.ParseDuration(since)
	if err != nil {
		return nil, fmt.Errorf("invalid duration format for --since: %w", err)
	}
	sinceTime := time.Now().Add(-sinceDuration)

	summary := &Summary{
		Timeframe: since,
	}

	for _, session := range sessions {
		if session.StartTime.Before(sinceTime) {
			continue
		}

		summary.TotalSessions++
		switch session.Status {
		case "completed":
			summary.SuccessfulSessions++
		case "error":
			summary.FailedSessions++
		case "running":
			summary.RunningSessions++
		}

		if session.AgentStateFile == "" {
			continue
		}

		agentState, err := loadAgentState(session.AgentStateFile)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("could not load agent state for session %s: %w", session.Name, err)
		}

		summary.TotalTokens += agentState.TokenUsage.TotalTokens
		summary.TotalPromptTokens += agentState.TokenUsage.TotalPromptTokens
		summary.TotalResponseTokens += agentState.TokenUsage.TotalResponseTokens
		summary.TotalCost += agent.CalculateCost(agentState.Model, agentState.TokenUsage)
	}

	if summary.TotalSessions > 0 {
		totalCompleted := summary.SuccessfulSessions + summary.FailedSessions
		if totalCompleted > 0 {
			summary.SuccessRate = (float64(summary.SuccessfulSessions) / float64(totalCompleted)) * 100
		}
	}

	return summary, nil
}

// displaySummary prints the summary statistics in a dashboard format.
func displaySummary(out io.Writer, summary *Summary) {
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "SESSION SUMMARY (Last %s)\n", summary.Timeframe)
	fmt.Fprintln(w, "---------------------------------")
	fmt.Fprintf(w, "Total Sessions:\t%d\n", summary.TotalSessions)
	fmt.Fprintf(w, "  - Successful:\t%d\n", summary.SuccessfulSessions)
	fmt.Fprintf(w, "  - Failed:\t%d\n", summary.FailedSessions)
	fmt.Fprintf(w, "  - Running:\t%d\n", summary.RunningSessions)
	fmt.Fprintf(w, "Success Rate:\t%.2f%%\n", summary.SuccessRate)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Token Usage:")
	fmt.Fprintf(w, "  Total Tokens:\t%d\n", summary.TotalTokens)
	fmt.Fprintf(w, "  Prompt Tokens:\t%d\n", summary.TotalPromptTokens)
	fmt.Fprintf(w, "  Response Tokens:\t%d\n", summary.TotalResponseTokens)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Cost:")
	fmt.Fprintf(w, "  Estimated Cost:\t$%.4f\n", summary.TotalCost)
	fmt.Fprintln(w, "")
	w.Flush()
}
