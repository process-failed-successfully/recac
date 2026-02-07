package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect [session-name]",
	Short: "Inspect the internal state of an agent session",
	Long:  `Displays detailed information about an agent's internal state, including memory, history, and token usage. If no session name is provided, it inspects the most recently started session.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOutput, _ := cmd.Flags().GetBool("json")
		historyLines, _ := cmd.Flags().GetInt("history")

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		var sessionName string
		if len(args) == 0 {
			// Find most recent session
			sessions, err := sm.ListSessions()
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}
			if len(sessions) == 0 {
				return fmt.Errorf("no sessions found")
			}
			// Sort by StartTime descending
			sort.Slice(sessions, func(i, j int) bool {
				return sessions[i].StartTime.After(sessions[j].StartTime)
			})
			sessionName = sessions[0].Name
			if !jsonOutput {
				fmt.Fprintf(cmd.OutOrStdout(), "Inspecting most recent session: %s\n", sessionName)
			}
		} else {
			sessionName = args[0]
		}

		session, err := sm.LoadSession(sessionName)
		if err != nil {
			return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
		}

		if session.AgentStateFile == "" {
			return fmt.Errorf("session '%s' has no agent state file recorded", sessionName)
		}

		// Load agent state using the mockable helper
		state, err := loadAgentState(session.AgentStateFile)
		if err != nil {
			return fmt.Errorf("failed to load agent state from %s: %w", session.AgentStateFile, err)
		}

		if jsonOutput {
			data, err := json.MarshalIndent(state, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal state to JSON: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		}

		// Human-readable output
		fmt.Fprintf(cmd.OutOrStdout(), "Session: %s (Status: %s)\n", session.Name, session.Status)
		fmt.Fprintf(cmd.OutOrStdout(), "Model: %s\n", state.Model)

		lastActivity := "Never"
		if !state.LastActivity.IsZero() {
			lastActivity = fmt.Sprintf("%s (%s ago)", state.LastActivity.Format(time.RFC3339), time.Since(state.LastActivity).Round(time.Second))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Last Activity: %s\n", lastActivity)

		fmt.Fprintf(cmd.OutOrStdout(), "Tokens: %d / %d (Prompt: %d, Response: %d)\n",
			state.TokenUsage.TotalTokens, state.MaxTokens,
			state.TokenUsage.TotalPromptTokens, state.TokenUsage.TotalResponseTokens)

		if len(state.Memory) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "\nMemory Items:")
			for i, mem := range state.Memory {
				fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, mem)
			}
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "\nMemory: (empty)")
		}

		fmt.Fprintln(cmd.OutOrStdout(), "\nRecent History:")
		if len(state.History) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "  (empty)")
		} else {
			start := 0
			if len(state.History) > historyLines {
				start = len(state.History) - historyLines
				fmt.Fprintf(cmd.OutOrStdout(), "  ... (skipping %d older messages)\n", start)
			}

			for i := start; i < len(state.History); i++ {
				msg := state.History[i]
				role := msg.Role
				content := msg.Content
				// Truncate content for display if too long
				if len(content) > 100 {
					content = content[:97] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", role, content)
			}
		}

		return nil
	},
}

func init() {
	inspectCmd.Flags().Bool("json", false, "Output in JSON format")
	inspectCmd.Flags().Int("history", 5, "Number of recent history messages to show")
	rootCmd.AddCommand(inspectCmd)
}
