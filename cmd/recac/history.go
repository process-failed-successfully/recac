package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var askOne = survey.AskOne

// initHistoryCmd initializes the history command and adds it to the root command.
func initHistoryCmd(rootCmd *cobra.Command) {
	historyCmd := &cobra.Command{
		Use:   "history [session-name]",
		Short: "Show history of a specific session or a list of all sessions",
		Long: `Displays detailed history for a specific RECAC session.
If no session name is provided, it lists all sessions and prompts for a selection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			sm, err := sessionManagerFactory()
			if err != nil {
				return fmt.Errorf("failed to initialize session manager: %w", err)
			}

			if len(args) > 0 {
				sessionName := args[0]
				session, err := sm.LoadSession(sessionName)
				if err != nil {
					return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
				}
				return displaySessionDetail(cmd, session)
			}
			return runInteractiveHistory(cmd, sm)
		},
	}
	rootCmd.AddCommand(historyCmd)
}

// runInteractiveHistory handles the interactive session selection.
func runInteractiveHistory(cmd *cobra.Command, sm ISessionManager) error {
	sessions, err := sm.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		cmd.Println("No sessions found.")
		return nil
	}

	// Sort sessions by start time, descending
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})

	var displaySessions []string
	sessionMap := make(map[string]*runner.SessionState)
	for _, s := range sessions {
		status := s.Status
		if s.Status == "running" && !sm.IsProcessRunning(s.PID) {
			status = "completed"
		}
		// Format for display: "SessionName (Status | 2023-10-27 10:00)"
		display := fmt.Sprintf("%-30s (%-10s | %s)", s.Name, status, s.StartTime.Format("2006-01-02 15:04"))
		displaySessions = append(displaySessions, display)
		sessionMap[display] = s
	}

	var selectedDisplay string
	prompt := &survey.Select{
		Message: "Select a session to view its history:",
		Options: displaySessions,
		PageSize: 15,
	}
	if err := askOne(prompt, &selectedDisplay); err != nil {
		if err.Error() == "interrupt" {
			return nil // User cancelled
		}
		return fmt.Errorf("failed to select session: %w", err)
	}

	selectedSession := sessionMap[selectedDisplay]
	return displaySessionDetail(cmd, selectedSession)
}

// displaySessionDetail prints a detailed view of a single session.
func displaySessionDetail(cmd *cobra.Command, session *runner.SessionState) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "Session Details for '%s'\n", session.Name)
	fmt.Fprintf(w, "-------------------------\n")
	fmt.Fprintf(w, "Name:\t%s\n", session.Name)
	fmt.Fprintf(w, "Status:\t%s\n", session.Status)
	fmt.Fprintf(w, "PID:\t%d\n", session.PID)
	fmt.Fprintf(w, "Type:\t%s\n", session.Type)
	fmt.Fprintf(w, "Start Time:\t%s\n", session.StartTime.Format(time.RFC1123))
	if !session.EndTime.IsZero() {
		fmt.Fprintf(w, "End Time:\t%s\n", session.EndTime.Format(time.RFC1123))
		fmt.Fprintf(w, "Duration:\t%s\n", session.EndTime.Sub(session.StartTime).Round(time.Second))
	}
	fmt.Fprintf(w, "Workspace:\t%s\n", session.Workspace)
	fmt.Fprintf(w, "Log File:\t%s\n", session.LogFile)
	if session.Error != "" {
		fmt.Fprintf(w, "Error:\t%s\n", session.Error)
	}
	w.Flush()

	// Agent State and Cost
	if session.AgentStateFile != "" {
		agentState, err := loadAgentState(session.AgentStateFile)
		if err == nil {
			fmt.Println("\nAgent & Token Usage")
			fmt.Println("-------------------")
			fmt.Fprintf(w, "Model:\t%s\n", agentState.Model)
			fmt.Fprintf(w, "Prompt Tokens:\t%d\n", agentState.TokenUsage.TotalPromptTokens)
			fmt.Fprintf(w, "Completion Tokens:\t%d\n", agentState.TokenUsage.TotalResponseTokens)
			fmt.Fprintf(w, "Total Tokens:\t%d\n", agentState.TokenUsage.TotalTokens)

			cost := agent.CalculateCost(agentState.Model, agentState.TokenUsage)
			fmt.Fprintf(w, "Estimated Cost:\t$%.6f\n", cost)
			w.Flush()
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "\nCould not load agent state from %s: %v\n", session.AgentStateFile, err)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\nNo agent state file associated with this session.")
	}


	// Display last 10 lines of the log file
	if _, err := os.Stat(session.LogFile); err == nil {
		fmt.Fprintf(w, "\nRecent Logs (last 10 lines)\n")
		fmt.Fprintf(w, "---------------------------\n")
		w.Flush()

		// In a real scenario, we might use a more robust method to get last N lines,
		// but for a CLI tool, this is often sufficient.
		logContent, err := os.ReadFile(session.LogFile)
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(logContent)), "\n")
			start := 0
			if len(lines) > 10 {
				start = len(lines) - 10
			}
			for _, line := range lines[start:] {
				cmd.Println(line)
			}
		} else {
			cmd.PrintErrf("Failed to read log file: %v\n", err)
		}
	}


	return nil
}

// loadAgentState reads and decodes the agent state file.
func loadAgentState(filePath string) (agent.State, error) {
	var state agent.State
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return state, fmt.Errorf("agent state file not found at %s", filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return state, fmt.Errorf("failed to read agent state file: %w", err)
	}
	if len(data) == 0 {
		return state, errors.New("agent state file is empty")
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("failed to unmarshal agent state: %w", err)
	}
	return state, nil
}
