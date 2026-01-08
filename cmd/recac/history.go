package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/runner"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// initHistoryCmd initializes the history command and adds it to the root command.
func initHistoryCmd(rootCmd *cobra.Command) {
	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "Show history of completed RECAC sessions",
		Long:  `Displays a summary of all completed RECAC sessions with performance metrics.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistoryCmd(runner.NewSessionManager)
		},
	}
	rootCmd.AddCommand(historyCmd)
}

// runHistoryCmd contains the core logic for the history command.
// It accepts a factory function for SessionManager to allow for mocking in tests.
func runHistoryCmd(newSessionManager func() (*runner.SessionManager, error)) error {
	sm, err := newSessionManager()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}

	sessions, err := sm.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	completedSessions := []*runner.SessionState{}
	for _, s := range sessions {
		if s.Status != "running" {
			completedSessions = append(completedSessions, s)
		}
	}

	if len(completedSessions) == 0 {
		fmt.Println("No completed sessions found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tSTARTED\tTOTAL TOKENS\tESTIMATED COST ($)")
	fmt.Fprintln(w, "----\t------\t-------\t------------\t------------------")

	for _, s := range completedSessions {
		var totalTokens int
		var estimatedCost float64

		if s.AgentStateFile != "" {
			agentState, err := loadAgentState(s.AgentStateFile)
			if err == nil {
				totalTokens = agentState.TokenUsage.TotalTokens
				// NOTE: This is a simplified cost model and does not account for different model pricing.
				// It assumes a generic rate of $1.00 per 1,000,000 tokens.
				estimatedCost = float64(totalTokens) / 1_000_000.0
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%.4f\n",
			s.Name,
			s.Status,
			s.StartTime.Format("2006-01-02 15:04:05"),
			totalTokens,
			estimatedCost,
		)
	}

	return w.Flush()
}

// loadAgentState reads and decodes the agent state file.
func loadAgentState(filePath string) (agent.State, error) {
	var state agent.State
	data, err := os.ReadFile(filePath)
	if err != nil {
		return state, fmt.Errorf("failed to read agent state file: %w", err)
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("failed to unmarshal agent state: %w", err)
	}
	return state, nil
}
