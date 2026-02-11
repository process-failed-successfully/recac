package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/db"
	"strings"

	"github.com/spf13/cobra"
)

var replyCmd = &cobra.Command{
	Use:   "reply <session_name> [answer]",
	Short: "Reply to a pending agent question",
	Long: `Responds to a question asked by an agent via 'agent-bridge ask'.
If no answer is provided as an argument, it will display the question and prompt for input.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runReply,
}

func init() {
	rootCmd.AddCommand(replyCmd)
}

func runReply(cmd *cobra.Command, args []string) error {
	sessionName := args[0]
	var answer string
	if len(args) > 1 {
		answer = strings.Join(args[1:], " ")
	}

	sm, err := sessionManagerFactory()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}

	session, err := sm.LoadSession(sessionName)
	if err != nil {
		return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
	}

	// 1. Connect to DB
	// We infer DB path from workspace
	dbPath := filepath.Join(session.Workspace, ".recac.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database not found at %s", dbPath)
	}

	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer store.Close()

	// 2. Check for Question
	// The agent-bridge uses RECAC_PROJECT_ID env var, which usually maps to the session/project name.
	// We first try using the session name as the project ID.
	projectID := session.Name

	question, err := store.GetSignal(projectID, "QUESTION")
	if err != nil {
		// Try "default" if sessionName fails
		q2, err2 := store.GetSignal("default", "QUESTION")
		if err2 == nil && q2 != "" {
			projectID = "default"
			question = q2
		} else {
			return fmt.Errorf("failed to check for questions: %w", err)
		}
	}

	if question == "" {
		return fmt.Errorf("no pending question found for session '%s' (Project: %s)", sessionName, projectID)
	}

	// 3. Prompt if needed
	if answer == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "\n🤖 Agent Question:\n%s\n\n", question)
		fmt.Fprint(cmd.OutOrStdout(), "Your Answer > ")

		scanner := bufio.NewScanner(cmd.InOrStdin())
		if scanner.Scan() {
			answer = strings.TrimSpace(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
	}

	if answer == "" {
		return fmt.Errorf("answer cannot be empty")
	}

	// 4. Send Answer
	if err := store.SetSignal(projectID, "ANSWER", answer); err != nil {
		return fmt.Errorf("failed to send answer: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Answer sent to agent.\n")
	return nil
}
