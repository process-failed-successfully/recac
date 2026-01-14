package main

import (
	"fmt"
	"recac/internal/ui"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(psCmd)
	if psCmd.Flags().Lookup("costs") == nil {
		psCmd.Flags().BoolP("costs", "c", false, "Show token usage and cost information")
	}
	if psCmd.Flags().Lookup("sort") == nil {
		psCmd.Flags().String("sort", "time", "Sort sessions by 'cost', 'time', or 'name'")
	}
	if psCmd.Flags().Lookup("errors") == nil {
		psCmd.Flags().BoolP("errors", "e", false, "Show the first line of the error for failed sessions")
	}
	if psCmd.Flags().Lookup("remote") == nil {
		psCmd.Flags().Bool("remote", false, "Include remote Kubernetes pods in the list")
	}
	if psCmd.Flags().Lookup("status") == nil {
		psCmd.Flags().String("status", "", "Filter sessions by status (e.g., 'running', 'completed', 'error')")
	}
	if psCmd.Flags().Lookup("since") == nil {
		psCmd.Flags().String("since", "", "Filter sessions started after a specific duration (e.g., '1h', '30m') or timestamp ('2006-01-02')")
	}
	if psCmd.Flags().Lookup("show-diff") == nil {
		psCmd.Flags().Bool("show-diff", false, "Show git diff for the most recent or specified session")
	}
	if psCmd.Flags().Lookup("session") == nil {
		psCmd.Flags().String("session", "", "Specify a session for --show-diff")
	}
	if psCmd.Flags().Lookup("watch") == nil {
		psCmd.Flags().BoolP("watch", "w", false, "Enable real-time dashboard view")
	}
}

var psCmd = &cobra.Command{
	Use:     "ps",
	Aliases: []string{"list"},
	Short:   "List sessions",
	Long:    `List all active and completed local sessions and, optionally, remote Kubernetes pods.`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		watch, _ := cmd.Flags().GetBool("watch")
		if watch {
			return ui.StartPsDashboard(cmd.Flags(), GetUnifiedSessions)
		}

		allSessions, warnings, err := GetUnifiedSessions(cmd.Flags())
		if err != nil {
			return err
		}
		for _, w := range warnings {
			cmd.PrintErrln(w)
		}

		if len(allSessions) == 0 {
			cmd.Println("No sessions found.")
			return nil
		}

		// --- Print Output ---
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		showCosts, _ := cmd.Flags().GetBool("costs")
		header := "NAME\tSTATUS\tLOCATION\tSTARTED\tDURATION"
		if showCosts {
			header += "\tPROMPT_TOKENS\tCOMPLETION_TOKENS\tTOTAL_TOKENS\tCOST"
		}
		fmt.Fprintln(w, header)

		for _, s := range allSessions {
			started := s.StartTime.Format("2006-01-02 15:04:05")
			var duration string
			if s.EndTime.IsZero() {
				duration = time.Since(s.StartTime).Round(time.Second).String()
			} else {
				duration = s.EndTime.Sub(s.StartTime).Round(time.Second).String()
			}

			baseOutput := fmt.Sprintf("%s\t%s\t%s\t%s\t%s",
				s.Name, s.Status, s.Location, started, duration)

			if showCosts {
				if s.HasCost {
					fmt.Fprintf(w, "%s\t%d\t%d\t%d\t$%.6f\n",
						baseOutput, s.Tokens.TotalPromptTokens, s.Tokens.TotalResponseTokens, s.Tokens.TotalTokens, s.Cost)
				} else {
					fmt.Fprintf(w, "%s\tN/A\tN/A\tN/A\tN/A\n", baseOutput)
				}
			} else {
				fmt.Fprintf(w, "%s\n", baseOutput)
			}
		}

		if err := w.Flush(); err != nil {
			return err
		}

		// --- Handle --show-diff ---
		sm, err := sessionManagerFactory() // We still need the session manager for diffs
		if err != nil {
			return fmt.Errorf("failed to create session manager for diff: %w", err)
		}
		showDiff, _ := cmd.Flags().GetBool("show-diff")
		if showDiff {
			sessionName, _ := cmd.Flags().GetString("session")
			if sessionName == "" {
				// Find the most recent session if not specified
				if len(allSessions) > 0 {
					sessionName = allSessions[0].Name // Assumes default sort by time
				} else {
					return fmt.Errorf("no sessions available to diff")
				}
			}
			cmd.Println() // Add a newline for better formatting
			return handleSingleSessionDiff(cmd, sm, sessionName)
		}

		return nil
	},
}

func handleSingleSessionDiff(cmd *cobra.Command, sm ISessionManager, sessionName string) error {
	session, err := sm.LoadSession(sessionName)
	if err != nil {
		return fmt.Errorf("failed to load session %s: %w", sessionName, err)
	}

	if session.StartCommitSHA == "" {
		return fmt.Errorf("session '%s' does not have a start commit SHA recorded", sessionName)
	}

	endSHA, err := getSessionEndSHA(session)
	if err != nil {
		return err
	}

	gitClient := gitClientFactory()
	diff, err := gitClient.Diff(session.Workspace, session.StartCommitSHA, endSHA)
	if err != nil {
		return fmt.Errorf("failed to get git diff: %w", err)
	}

	cmd.Println(diff)
	return nil
}
