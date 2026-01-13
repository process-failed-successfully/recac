package main

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(psCmd)
	// Note: The `unifiedSession` struct is now in `session_utils.go`
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
}

var psCmd = &cobra.Command{
	Use:     "ps",
	Aliases: []string{"list"},
	Short:   "List sessions",
	Long:    `List all active and completed local sessions and, optionally, remote Kubernetes pods.`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Get Flags ---
		showRemote, _ := cmd.Flags().GetBool("remote")
		statusFilter, _ := cmd.Flags().GetString("status")
		sortBy, _ := cmd.Flags().GetString("sort")
		showCosts, _ := cmd.Flags().GetBool("costs")
		showErrors, _ := cmd.Flags().GetBool("errors")

		// --- Get Sessions using the new shared function ---
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		allSessions, err := getFullSessionList(sm, showRemote, statusFilter)
		if err != nil {
			return err // Error is already descriptive
		}

		if len(allSessions) == 0 {
			cmd.Println("No sessions found.")
			return nil
		}

		// --- Sort all sessions ---
		sort.SliceStable(allSessions, func(i, j int) bool {
			switch sortBy {
			case "cost":
				if allSessions[i].HasCost && allSessions[j].HasCost {
					return allSessions[i].Cost > allSessions[j].Cost
				}
				return allSessions[i].HasCost // Ones with cost come first
			case "name":
				return allSessions[i].Name < allSessions[j].Name
			case "time":
				fallthrough
			default:
				return allSessions[i].StartTime.After(allSessions[j].StartTime)
			}
		})

		// --- Print Output ---
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)

		// Build header dynamically
		header := []string{"NAME", "STATUS", "LOCATION", "STARTED", "DURATION"}
		if showCosts {
			header = append(header, "PROMPT_TOKENS", "COMPLETION_TOKENS", "TOTAL_TOKENS", "COST")
		}
		if showErrors {
			header = append(header, "ERROR")
		}
		fmt.Fprintln(w, strings.Join(header, "\t"))

		for _, s := range allSessions {
			started := s.StartTime.Format("2006-01-02 15:04:05")
			var duration string
			if s.EndTime.IsZero() {
				duration = time.Since(s.StartTime).Round(time.Second).String()
			} else {
				duration = s.EndTime.Sub(s.StartTime).Round(time.Second).String()
			}

			// Build row dynamically
			row := []string{s.Name, s.Status, s.Location, started, duration}
			if showCosts {
				if s.HasCost {
					row = append(row,
						fmt.Sprintf("%d", s.Tokens.TotalPromptTokens),
						fmt.Sprintf("%d", s.Tokens.TotalResponseTokens),
						fmt.Sprintf("%d", s.Tokens.TotalTokens),
						fmt.Sprintf("$%.6f", s.Cost),
					)
				} else {
					row = append(row, "N/A", "N/A", "N/A", "N/A")
				}
			}
			if showErrors {
				// Only show the first line of the error
				firstLine := ""
				if s.Error != "" {
					firstLine = strings.Split(s.Error, "\n")[0]
				}
				row = append(row, firstLine)
			}
			fmt.Fprintln(w, strings.Join(row, "\t"))
		}

		return w.Flush()
	},
}
