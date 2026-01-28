package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"recac/internal/utils"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	pruneDryRun bool
	pruneAll    bool
	pruneSince  string
)

func init() {
	pruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false, "Simulate pruning without deleting files")
	pruneCmd.Flags().BoolVar(&pruneAll, "all", false, "Prune all sessions, including running ones")
	pruneCmd.Flags().StringVar(&pruneSince, "since", "", "Prune sessions older than a duration (e.g., 7d, 24h)")
	rootCmd.AddCommand(pruneCmd)
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove session artifacts from disk",
	Long: `Prune removes session artifacts (logs, state files, workspaces, docker containers) for completed, stopped, or errored sessions.
Use the --all flag to remove all sessions, including running ones.
Use the --since flag to remove sessions older than a specific duration (e.g., '7d', '24h').
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

		// Initialize Docker client (best effort)
		dockerCli, err := dockerClientFactory("recac-prune")
		if err != nil {
			cmd.PrintErrf("Warning: Failed to initialize Docker client: %v\n", err)
			dockerCli = nil
		} else {
			defer dockerCli.Close()
		}

		var cutoff time.Time
		if pruneSince != "" {
			duration, err := utils.ParseDurationWithDays(pruneSince)
			if err != nil {
				return fmt.Errorf("invalid duration format for --since: %w", err)
			}
			cutoff = time.Now().Add(-duration)
		}

		var sessionsToPrune []*runner.SessionState
		for _, s := range sessions {
			// Determine if a session is a candidate for pruning based on its status.
			isCandidate := false
			if pruneAll {
				isCandidate = true
			} else {
				switch s.Status {
				case "completed", "stopped", "error":
					isCandidate = true
				}
			}

			// If it's a candidate, check if it meets the time filter (if one is specified).
			if isCandidate {
				if !cutoff.IsZero() {
					// Time filter is active; only prune if the session is old enough.
					if s.StartTime.Before(cutoff) {
						sessionsToPrune = append(sessionsToPrune, s)
					}
				} else {
					// No time filter; prune all candidates.
					sessionsToPrune = append(sessionsToPrune, s)
				}
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
			// 1. Cleanup Workspace
			if s.Workspace != "" {
				safe := false
				// Safety checks to prevent deleting user directories
				if filepath.IsAbs(s.Workspace) && s.Workspace != "/" {
					base := filepath.Base(s.Workspace)
					if strings.HasPrefix(base, "recac-agent-") || strings.Contains(s.Workspace, ".recac") {
						safe = true
					} else if strings.HasPrefix(s.Workspace, os.TempDir()) {
						safe = true
					}
				}

				if safe {
					if err := os.RemoveAll(s.Workspace); err != nil {
						// Only log error if file still exists (ignoring duplicate removal)
						if !os.IsNotExist(err) {
							cmd.PrintErrf("Failed to remove workspace %s: %v\n", s.Workspace, err)
						}
					}
				}
			}

			// 2. Cleanup Docker Container
			if s.ContainerID != "" && dockerCli != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := dockerCli.RemoveContainer(ctx, s.ContainerID, true); err != nil {
					// Check if error is "No such container" and ignore it
					if !strings.Contains(err.Error(), "No such container") {
						cmd.PrintErrf("Failed to remove container %s: %v\n", s.ContainerID, err)
					}
				}
				cancel()
			}

			// 3. Remove Session Files
			// Use RemoveSession with force=true to ensure cleanup even if status is stale
			if err := sm.RemoveSession(s.Name, true); err != nil {
				cmd.PrintErrln(err)
				continue
			}
			cmd.Printf("Pruned session: %s\n", s.Name)
			prunedCount++
		}

		cmd.Printf("\nPruned %d session(s).\n", prunedCount)

		return nil
	},
}
