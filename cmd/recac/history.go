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
		Use:   "history [session-name]",
		Short: "Show history of completed RECAC sessions",
		Long: `Displays a summary of all completed RECAC sessions or details of a specific session.
If a session name is provided, it will show detailed information for that session.
Otherwise, it will list all completed sessions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return runHistoryDetailCmd(runner.NewSessionManager, args[0])
			}
			return runHistoryListCmd(runner.NewSessionManager)
		},
	}
	rootCmd.AddCommand(historyCmd)
}

// runHistoryListCmd contains the core logic for listing all history sessions.
// It accepts a factory function for SessionManager to allow for mocking in tests.
func runHistoryListCmd(newSessionManager func() (*runner.SessionManager, error)) error {
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
				pricing, ok := agent.GetPricing(agentState.Model)
				if ok {
					cost := (float64(agentState.TokenUsage.PromptTokens)*pricing.PromptCost +
						float64(agentState.TokenUsage.CompletionTokens)*pricing.CompletionCost) / 1_000_000.0
					estimatedCost = cost
				}
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%.6f\n",
			s.Name,
			s.Status,
			s.StartTime.Format("2006-01-02 15:04:05"),
			totalTokens,
			estimatedCost,
		)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush tabwriter: %w", err)
	}
	return nil
}

// runHistoryDetailCmd contains the core logic for displaying a single session's details.
func runHistoryDetailCmd(newSessionManager func() (*runner.SessionManager, error), sessionName string) error {
	sm, err := newSessionManager()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}

	session, err := sm.LoadSession(sessionName)
	if err != nil {
		return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "Session Details for '%s':\n", session.Name)
	fmt.Fprintln(w, "------------------------")
	fmt.Fprintf(w, "Name:\t%s\n", session.Name)
	fmt.Fprintf(w, "Status:\t%s\n", session.Status)
	fmt.Fprintf(w, "PID:\t%d\n", session.PID)
	fmt.Fprintf(w, "Started:\t%s\n", session.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "Workspace:\t%s\n", session.Workspace)
	fmt.Fprintf(w, "Log File:\t%s\n", session.LogFile)

	if session.AgentStateFile != "" {
		agentState, err := loadAgentState(session.AgentStateFile)
		if err != nil {
			fmt.Fprintf(w, "Agent State:\tError loading state: %v\n", err)
		} else {
			printAgentStateDetails(w, agentState)
		}
	} else {
		fmt.Fprintln(w, "Agent State:\tNot available")
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush tabwriter: %w", err)
	}
	return nil
}

// printAgentStateDetails formats and prints the agent state information.
func printAgentStateDetails(w *tabwriter.Writer, state agent.State) {
	fmt.Fprintln(w, "\nAgent Performance:")
	fmt.Fprintln(w, "------------------")
	fmt.Fprintf(w, "Model:\t%s\n", state.Model)
	fmt.Fprintf(w, "Prompt Tokens:\t%d\n", state.TokenUsage.PromptTokens)
	fmt.Fprintf(w, "Completion Tokens:\t%d\n", state.TokenUsage.CompletionTokens)
	fmt.Fprintf(w, "Total Tokens:\t%d\n", state.TokenUsage.TotalTokens)

	// Fetch pricing and calculate cost
	pricing, ok := agent.GetPricing(state.Model)
	var estimatedCost float64
	if ok {
		estimatedCost = (float64(state.TokenUsage.PromptTokens)*pricing.PromptCost) + (float64(state.TokenUsage.CompletionTokens)*pricing.CompletionCost)
		estimatedCost /= 1_000_000 // Costs are per million tokens
	}
	fmt.Fprintf(w, "Estimated Cost ($):\t%.6f\n", estimatedCost)

	if state.FinalError != "" {
		fmt.Fprintf(w, "Final Error:\t%s\n", state.FinalError)
	}
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
