package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"recac/internal/agent"
	"recac/internal/runner"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// initHistoryCmd initializes the history command and adds it to the root command.
func initHistoryCmd(rootCmd *cobra.Command) {
	var (
		status string
		sortBy string
		limit  int
	)

	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "Show history of completed RECAC sessions",
		Long:  `Displays a summary of all completed RECAC sessions with performance metrics.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			sm, err := sessionManagerFactory()
			if err != nil {
				return fmt.Errorf("failed to initialize session manager: %w", err)
			}
			return runHistoryCmd(cmd, sm, status, sortBy, limit)
		},
	}

	historyCmd.Flags().StringVar(&status, "status", "", "Filter by session status (e.g., completed, failed)")
	historyCmd.Flags().StringVar(&sortBy, "sort-by", "", "Sort by column (e.g., cost, tokens)")
	historyCmd.Flags().IntVar(&limit, "limit", 0, "Limit the number of results")

	rootCmd.AddCommand(historyCmd)
}

// runHistoryCmd contains the core logic for the history command.
// It accepts an ISessionManager for testability.
func runHistoryCmd(cmd *cobra.Command, sm ISessionManager, status, sortBy string, limit int) error {

	sessions, err := sm.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	type sessionInfo struct {
		*runner.SessionState
		totalTokens   int
		estimatedCost float64
	}

	var processedSessions []sessionInfo

	for _, s := range sessions {
		if s.Status == "running" {
			continue
		}

		// Status filtering
		if status != "" && s.Status != status {
			continue
		}

		var totalTokens int
		var estimatedCost float64
		if s.AgentStateFile != "" {
			agentState, err := loadAgentState(s.AgentStateFile)
			if err == nil {
				totalTokens = agentState.TokenUsage.TotalTokens
				estimatedCost = agent.CalculateCost(agentState.Model, agentState.TokenUsage)
			}
		}
		processedSessions = append(processedSessions, sessionInfo{s, totalTokens, estimatedCost})
	}

	// Sorting
	if sortBy != "" {
		sort.Slice(processedSessions, func(i, j int) bool {
			switch sortBy {
			case "cost":
				return processedSessions[i].estimatedCost > processedSessions[j].estimatedCost
			case "tokens":
				return processedSessions[i].totalTokens > processedSessions[j].totalTokens
			default:
				// Default sort by start time, descending
				return processedSessions[i].StartTime.After(processedSessions[j].StartTime)
			}
		})
	}

	// Limiting
	if limit > 0 && len(processedSessions) > limit {
		processedSessions = processedSessions[:limit]
	}

	if len(processedSessions) == 0 {
		cmd.Println("No completed sessions found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tSTARTED\tTOTAL TOKENS\tESTIMATED COST ($)")
	fmt.Fprintln(w, "----\t------\t-------\t------------\t------------------")

	for _, s := range processedSessions {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%.4f\n",
			s.Name,
			s.Status,
			s.StartTime.Format("2006-01-02 15:04:05"),
			s.totalTokens,
			s.estimatedCost,
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
