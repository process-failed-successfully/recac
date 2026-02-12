package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"recac/internal/agent"
	"recac/internal/db"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

var forceRewind bool

func init() {
	rewindCmd.Flags().BoolVarP(&forceRewind, "force", "f", false, "Force rewind without confirmation")
	rootCmd.AddCommand(rewindCmd)
}

var rewindCmd = &cobra.Command{
	Use:   "rewind [session-name] [iteration]",
	Short: "Rewind a session to a specific iteration",
	Long: `Rewind a session to a previous state (iteration).
This command will:
1. Checkout the git commit associated with the specified iteration.
2. Reset the workspace to that commit (hard reset).
3. Delete all DB observations created after that iteration.
4. Update the agent state to resume from that iteration.

WARNING: This is a destructive operation. Any progress after the specified iteration will be lost.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]
		targetIteration, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid iteration number: %w", err)
		}

		if targetIteration < 0 {
			return fmt.Errorf("iteration number must be non-negative")
		}

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		// Load session
		session, err := sm.LoadSession(sessionName)
		if err != nil {
			return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
		}

		// Check if running
		if session.Status == "running" && sm.IsProcessRunning(session.PID) {
			return fmt.Errorf("session '%s' is currently running. Please stop it before rewinding.", sessionName)
		}

		workspace := session.Workspace
		if workspace == "" {
			return fmt.Errorf("session has no workspace defined")
		}

		// Initialize Git Client
		gitClient := gitClientFactory()

		// Find Commit for Iteration
		// We look for "chore: progress update (iteration X)"
		// We use git log --grep
		// Note: IGitClient.Log returns []string (lines), not raw output.
		// But IGitClient.Run returns (string, error). We use Run for flexibility.
		output, err := gitClient.Run(workspace, "log", "--grep", fmt.Sprintf("chore: progress update (iteration %d)", targetIteration), "-n", "1", "--format=%H|%ct")
		if err != nil {
			return fmt.Errorf("failed to search git log: %w", err)
		}

		output = strings.TrimSpace(output)
		if output == "" {
			return fmt.Errorf("could not find commit for iteration %d. Make sure the iteration exists and was committed.", targetIteration)
		}

		parts := strings.Split(output, "|")
		if len(parts) != 2 {
			return fmt.Errorf("unexpected git log output: %s", output)
		}
		commitHash := parts[0]
		commitTimeUnix, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse commit time: %w", err)
		}
		commitTime := time.Unix(commitTimeUnix, 0)

		fmt.Printf("Found checkpoint for iteration %d:\n", targetIteration)
		fmt.Printf("  Commit:    %s\n", commitHash)
		fmt.Printf("  Timestamp: %s\n", commitTime.Format(time.RFC3339))
		fmt.Printf("  Workspace: %s\n", workspace)

		if !forceRewind {
			fmt.Print("\nWARNING: This will delete all progress (code and memory) after this point.\nAre you sure you want to continue? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(response)) != "y" {
				fmt.Println("Rewind cancelled.")
				return nil
			}
		}

		// 1. Git Reset
		fmt.Println("Resetting workspace...")
		// Use Run instead of Checkout/ResetHard because interface might not have them or we want precise control
		if _, err := gitClient.Run(workspace, "checkout", commitHash); err != nil {
			return fmt.Errorf("failed to checkout commit: %w", err)
		}
		// Hard reset to ensure clean state
		if _, err := gitClient.Run(workspace, "reset", "--hard", commitHash); err != nil {
			return fmt.Errorf("failed to hard reset: %w", err)
		}
		// Clean untracked files
		if _, err := gitClient.Run(workspace, "clean", "-fd"); err != nil {
			return fmt.Errorf("failed to clean workspace: %w", err)
		}

		// 2. DB Rewind
		dbType := os.Getenv("RECAC_DB_TYPE")
		dbURL := os.Getenv("RECAC_DB_URL")
		if dbType == "" {
			dbType = "sqlite"
			if dbURL == "" {
				dbURL = filepath.Join(workspace, ".recac.db")
			}
		} else if dbType == "sqlite" && dbURL == "" {
			dbURL = filepath.Join(workspace, ".recac.db")
		}

		storeConfig := db.StoreConfig{
			Type:             dbType,
			ConnectionString: dbURL,
		}

		dbStore, err := db.NewStore(storeConfig)
		if err != nil {
			fmt.Printf("Warning: Failed to connect to DB: %v. Skipping DB rewind.\n", err)
		} else {
			defer dbStore.Close()
			// Use session name as project ID
			projectID := sessionName

			fmt.Printf("Truncating DB observations for project '%s' after %s...\n", projectID, commitTime)
			if err := dbStore.DeleteObservationsAfter(projectID, commitTime); err != nil {
				return fmt.Errorf("failed to delete observations: %w", err)
			}
		}

		// 3. Update Agent State
		agentStateFile := filepath.Join(workspace, ".agent_state.json")
		if session.AgentStateFile != "" {
			agentStateFile = session.AgentStateFile
		}

		fmt.Printf("Updating agent state in %s...\n", agentStateFile)

		var state agent.State
		// Start with empty/default if file missing
		if err := runner.LoadSafeguardedState(agentStateFile, &state); err != nil {
			fmt.Printf("Warning: Failed to load agent state: %v. Creating new state.\n", err)
		}

		// Update Iteration
		state.Iteration = targetIteration

		// Update UpdatedAt
		state.UpdatedAt = time.Now()

		// Save back
		smState := agent.NewStateManager(agentStateFile)
		if err := smState.Save(state); err != nil {
			return fmt.Errorf("failed to save agent state: %w", err)
		}

		fmt.Printf("Successfully rewound session '%s' to iteration %d.\n", sessionName, targetIteration)
		return nil
	},
}
