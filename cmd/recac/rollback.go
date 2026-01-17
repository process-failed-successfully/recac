package main

import (
	"fmt"
	"recac/internal/git"
	"recac/internal/runner"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	rollbackSteps int
	rollbackList  bool
	rollbackForce bool
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [SESSION_NAME]",
	Short: "Rollback session to a previous iteration",
	Long: `Undo changes made by the autonomous agent by reverting the project state to a previous checkpoint.
Each "progress update" commit is considered a checkpoint.

You can specify the number of steps to rollback (default 1) or view the list of available checkpoints.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		var session *runner.SessionState
		if len(args) == 1 {
			session, err = sm.LoadSession(args[0])
			if err != nil {
				return err
			}
		} else {
			// Find most recent session
			sessions, err := sm.ListSessions()
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}
			if len(sessions) == 0 {
				return fmt.Errorf("no sessions found")
			}
			sort.Slice(sessions, func(i, j int) bool {
				return sessions[i].StartTime.After(sessions[j].StartTime)
			})
			session = sessions[0]
			fmt.Fprintf(cmd.OutOrStdout(), "Using most recent session: %s\n", session.Name)
		}

		// 1. Ensure Session is Stopped (Safety)
		if sm.IsProcessRunning(session.PID) && !rollbackForce {
			return fmt.Errorf("session '%s' is currently running (PID %d). Please stop it first with 'recac stop %s' or use --force (risky)", session.Name, session.PID, session.Name)
		}

		workspace := session.Workspace
		if workspace == "" {
			return fmt.Errorf("session workspace not found")
		}

		// 2. Query Git Log
		gitClient := git.NewClient()
		// We look for commits with "chore: progress update (iteration X)"
		// We get full log to parse correctly
		logOutput, err := gitClient.Log(workspace, "--oneline", "--no-abbrev-commit")
		if err != nil {
			return fmt.Errorf("failed to get git log: %w", err)
		}

		type checkpoint struct {
			SHA       string
			Iteration string
			Line      string
		}

		var checkpoints []checkpoint
		lines := strings.Split(logOutput, "\n")
		// Regex to match our automated commits
		// chore: progress update (iteration 5)
		// Matches full SHA at start of line
		re := regexp.MustCompile(`^([a-f0-9]+)\s+.*chore: progress update \(iteration (\d+)\)`)

		for _, line := range lines {
			line = strings.TrimSpace(line)
			matches := re.FindStringSubmatch(line)
			if len(matches) == 3 {
				checkpoints = append(checkpoints, checkpoint{
					SHA:       matches[1],
					Iteration: matches[2],
					Line:      line,
				})
			}
		}

		if len(checkpoints) == 0 {
			return fmt.Errorf("no rollback checkpoints found in git history")
		}

		// 3. List Mode
		if rollbackList {
			fmt.Fprintf(cmd.OutOrStdout(), "Available checkpoints for session '%s':\n", session.Name)
			for _, cp := range checkpoints {
				fmt.Fprintln(cmd.OutOrStdout(), cp.Line)
			}
			return nil
		}

		// 4. Determine Target
		if rollbackSteps < 1 {
			return fmt.Errorf("steps must be >= 1")
		}
		if rollbackSteps >= len(checkpoints) {
			// Actually if steps == len, we go to "before first iteration"?
			// The list is all checkpoints.
			// If we have 3 checkpoints: C3, C2, C1.
			// rollback 1 -> C2.
			// rollback 2 -> C1.
			// rollback 3 -> before C1?
			// But checkpoints list only contains "progress update" commits.
			// If we want to rollback to a state, we reset to that SHA.
			// So "rollback 1 step" means "Reset to the state BEFORE the last step" or "Reset to the previous step".
			// If I am at C3. I want C2. That is index 1.

			// Let's assume the user wants to revert to checkpoints[index].
			return fmt.Errorf("cannot rollback %d steps (only %d checkpoints available)", rollbackSteps, len(checkpoints))
		}

		target := checkpoints[rollbackSteps]

		// 5. Execute Rollback
		fmt.Fprintf(cmd.OutOrStdout(), "Rolling back %d step(s) to iteration %s (%s)...\n", rollbackSteps, target.Iteration, target.SHA)

		if err := gitClient.Reset(workspace, target.SHA); err != nil {
			return fmt.Errorf("rollback failed: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Rollback successful.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
	rollbackCmd.Flags().IntVarP(&rollbackSteps, "steps", "n", 1, "Number of steps (checkpoints) to rollback")
	rollbackCmd.Flags().BoolVarP(&rollbackList, "list", "l", false, "List available checkpoints")
	rollbackCmd.Flags().BoolVar(&rollbackForce, "force", false, "Force rollback even if session is running")
}
