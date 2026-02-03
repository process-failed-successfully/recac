package main

import (
	"fmt"
	"path/filepath"

	"recac/internal/db"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(pokeCmd)
}

var pokeCmd = &cobra.Command{
	Use:   "poke [session-name] [message]",
	Short: "Inject a hint or instruction into a running session",
	Long:  `Injects a message into the running session's context. The agent will see this message in its next iteration.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]
		message := args[1]

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		session, err := sm.LoadSession(sessionName)
		if err != nil {
			return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
		}

		// Determine DB Path
		// Default: .recac.db in workspace
		dbPath := filepath.Join(session.Workspace, ".recac.db")

		dbConfig := db.StoreConfig{
			Type:             "sqlite", // Assuming sqlite for local/detached sessions
			ConnectionString: dbPath,
		}

		store, err := db.NewStore(dbConfig)
		if err != nil {
			return fmt.Errorf("failed to connect to session database at %s: %w", dbPath, err)
		}
		defer store.Close()

		projectID := session.Project
		if projectID == "" {
			projectID = session.Name // Fallback
		}

		// Set the signal
		if err := store.SetSignal(projectID, "USER_HINT", message); err != nil {
			return fmt.Errorf("failed to set hint signal: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully poked session '%s' with message: \"%s\"\n", sessionName, message)
		return nil
	},
}
