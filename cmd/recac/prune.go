package main

import (
	"fmt"
	"os"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

var (
	pruneDryRun bool
	pruneAll    bool
)

func init() {
	pruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false, "Simulate pruning without deleting files")
	pruneCmd.Flags().BoolVar(&pruneAll, "all", false, "Prune all sessions, including running ones")
	rootCmd.AddCommand(pruneCmd)
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove session artifacts from disk",
	Long: `Prune removes session artifacts (logs, state files) for completed, stopped, or errored sessions.
Use the --all flag to remove all sessions, including running ones.
Use the --dry-run flag to see which sessions would be pruned without deleting them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		var sessionsToPrune []*runner.SessionState
		for _, s := range sessions {
			if pruneAll {
				sessionsToPrune = append(sessionsToPrune, s)
				continue
			}

			switch s.Status {
			case "completed", "stopped", "error":
				sessionsToPrune = append(sessionsToPrune, s)
			}
		}

		if len(sessionsToPrune) == 0 {
			cmd.Println("No sessions to prune.")
			return nil
		}

		if pruneDryRun {
			cmd.Println("Dry run enabled. The following sessions would be pruned:")
			for _, s := range sessionsToPrune {
				cmd.Printf("- %s (status: %s)\n", s.Name, s.Status)
			}
			return nil
		}

		prunedCount := 0
		for _, s := range sessionsToPrune {
			jsonPath := sm.GetSessionPath(s.Name)
			logPath := s.LogFile

			errs := []error{}
			if err := os.Remove(jsonPath); err != nil && !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("failed to remove session file %s: %w", jsonPath, err))
			}
			if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("failed to remove log file %s: %w", logPath, err))
			}

			if len(errs) > 0 {
				for _, e := range errs {
					cmd.PrintErrln(e)
				}
				continue
			}
			cmd.Printf("Pruned session: %s\n", s.Name)
			prunedCount++
		}

		cmd.Printf("\nPruned %d session(s).\n", prunedCount)

		return nil
	},
}
