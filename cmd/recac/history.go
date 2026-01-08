package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/runner"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	// ErrSessionNotFound is returned when a session is not found.
	ErrSessionNotFound = fmt.Errorf("session not found")
)

// initHistoryCmd initializes the history command and adds it to the root command.
func initHistoryCmd(rootCmd *cobra.Command) {
	historyCmd := &cobra.Command{
		Use:   "history [session-name]",
		Short: "Show history of completed RECAC sessions",
		Long: `Displays a summary of all completed RECAC sessions.
If a session-name is provided, it shows a detailed view of that specific session.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Pass args to the handler
			return runHistoryCmd(runner.NewSessionManager, args)
		},
	}
	rootCmd.AddCommand(historyCmd)
}

// runHistoryCmd contains the core logic for the history command.
// It accepts a factory function for SessionManager to allow for mocking in tests.
func runHistoryCmd(newSessionManager func() (*runner.SessionManager, error), args []string) error {
	sm, err := newSessionManager()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}

	// Route to detail view if a session name is provided
	if len(args) == 1 {
		sessionName := args[0]
		session, err := sm.LoadSession(sessionName)
		if err != nil {
			// Return a specific error for not found
			return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionName)
		}
		return displaySessionDetail(session)
	}

	if len(args) > 1 {
		return fmt.Errorf("too many arguments, expected 0 or 1")
	}

	// Default to list view
	return displaySessionList(sm)
}

// displaySessionList prints a table of all completed sessions.
func displaySessionList(sm *runner.SessionManager) error {
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
				cost, err := agent.CalculateCost(agentState.Model, agentState.TokenUsage.PromptTokens, agentState.TokenUsage.CompletionTokens)
				if err == nil {
					estimatedCost = cost
				}
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

// displaySessionDetail prints a detailed view of a single session.
func displaySessionDetail(session *runner.SessionState) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "SESSION DETAILS")
	fmt.Fprintln(w, "---------------")
	fmt.Fprintf(w, "Name:\t%s\n", session.Name)
	fmt.Fprintf(w, "Status:\t%s\n", session.Status)
	fmt.Fprintf(w, "Start Time:\t%s\n", session.StartTime.Format(time.RFC1123))
	fmt.Fprintf(w, "End Time:\t%s\n", session.EndTime.Format(time.RFC1123))
	fmt.Fprintf(w, "Duration:\t%s\n", session.EndTime.Sub(session.StartTime).Round(time.Second))

	if session.AgentStateFile != "" {
		agentState, err := loadAgentState(session.AgentStateFile)
		if err != nil {
			fmt.Fprintf(w, "Agent State:\tERROR (%v)\n", err)
		} else {
			fmt.Fprintln(w, "\nAGENT & TOKEN INFO")
			fmt.Fprintln(w, "------------------")
			fmt.Fprintf(w, "Model:\t%s\n", agentState.Model)
			fmt.Fprintf(w, "Prompt Tokens:\t%d\n", agentState.TokenUsage.PromptTokens)
			fmt.Fprintf(w, "Completion Tokens:\t%d\n", agentState.TokenUsage.CompletionTokens)
			fmt.Fprintf(w, "Total Tokens:\t%d\n", agentState.TokenUsage.TotalTokens)

			estimatedCost, err := agent.CalculateCost(agentState.Model, agentState.TokenUsage.PromptTokens, agentState.TokenUsage.CompletionTokens)
			if err != nil {
				fmt.Fprintf(w, "Estimated Cost ($):\tERROR (%v)\n", err)
			} else {
				fmt.Fprintf(w, "Estimated Cost ($):\t%.4f\n", estimatedCost)
			}
		}
	}

	if session.Error != "" {
		fmt.Fprintln(w, "\nFINAL ERROR")
		fmt.Fprintln(w, "-----------")
		fmt.Fprintf(w, "%s\n", session.Error)
	}

	return nil
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
