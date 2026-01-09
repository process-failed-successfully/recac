package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	psCmd.Flags().Bool("errors", false, "Show the first line of the error message for failed sessions")
	rootCmd.AddCommand(psCmd)
}

var psCmd = &cobra.Command{
	Use:     "ps",
	Aliases: []string{"list"},
	Short:   "List all sessions",
	Long:    `List all active and completed sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		if len(sessions) == 0 {
			cmd.Println("No sessions found.")
			return nil
		}

		showErrors, _ := cmd.Flags().GetBool("errors")

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		header := "NAME\tSTATUS\tSTARTED\tDURATION"
		if showErrors {
			header += "\tERROR"
		}
		fmt.Fprintln(w, header)

		for _, session := range sessions {
			started := session.StartTime.Format("2006-01-02 15:04:05")
			var duration string
			if session.EndTime.IsZero() {
				duration = time.Since(session.StartTime).Round(time.Second).String()
			} else {
				duration = session.EndTime.Sub(session.StartTime).Round(time.Second).String()
			}

			line := fmt.Sprintf("%s\t%s\t%s\t%s",
				session.Name,
				session.Status,
				started,
				duration,
			)

			if showErrors {
				var errorMsg string
				if session.Error != "" {
					// Get the first line of the error
					errorMsg = strings.Split(session.Error, "\n")[0]
				}
				line += fmt.Sprintf("\t%s", errorMsg)
			}
			fmt.Fprintln(w, line)
		}

		return w.Flush()
	},
}
