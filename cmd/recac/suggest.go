package main

import (
	"fmt"
	"recac/internal/runner"
	"strings"

	"github.com/spf13/cobra"
)

// Suggestion holds a suggested command and its rationale.
type Suggestion struct {
	Command     string
	Description string
	Reason      string
}

func init() {
	rootCmd.AddCommand(suggestCmd)
}

var suggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Suggests context-aware commands to run next",
	Long: `Analyzes the current git status and session states to provide a list of
relevant and useful recac commands to run next. This helps in discovering useful
commands and speeding up your workflow.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Client Setup ---
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}
		gitClient := gitClientFactory()

		// --- Gather Context ---
		// For git status, we check the current working directory.
		// A more robust implementation might find the repo root.
		isDirty, err := gitClient.IsDirty(".")
		if err != nil {
			// Not a git repo or git not found, we can still suggest session commands
			isDirty = false
		}

		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("could not list sessions: %w", err)
		}

		// --- Generate Suggestions ---
		suggestions := generateSuggestions(isDirty, sessions)

		// --- Print Output ---
		if len(suggestions) == 0 {
			cmd.Println("✨ All clean! No specific suggestions right now. Try 'recac start' to begin a new session.")
			return nil
		}

		cmd.Println("Here are some suggested commands based on the current context:")
		cmd.Println()
		for _, s := range suggestions {
			cmd.Printf("👉 %s\n", s.Command)
			cmd.Printf("   Reason: %s\n\n", s.Reason)
		}

		return nil
	},
}

// generateSuggestions is the core logic for deciding which commands to suggest.
func generateSuggestions(isDirty bool, sessions []*runner.SessionState) []Suggestion {
	var suggestions []Suggestion

	// --- Git-based Suggestions ---
	if isDirty {
		suggestions = append(suggestions, Suggestion{
			Command: "recac start --goal \"<your goal>\"",
			Reason:  "You have uncommitted changes. You could start a new session to commit them.",
		})
	}

	// --- Session-based Suggestions ---
	runningSessions := getSessionsByStatus(sessions, "running")
	if len(runningSessions) > 0 {
		suggestions = append(suggestions, Suggestion{
			Command: "recac ps",
			Reason:  fmt.Sprintf("You have %d running session(s). Check their status.", len(runningSessions)),
		})
		suggestions = append(suggestions, Suggestion{
			Command: fmt.Sprintf("recac attach %s", runningSessions[0].Name),
			Reason:  fmt.Sprintf("Attach to your running session '%s' to see live output.", runningSessions[0].Name),
		})
	}

	completedSessions := getSessionsByStatus(sessions, "completed")
	if len(completedSessions) > 0 {
		suggestions = append(suggestions, Suggestion{
			Command: fmt.Sprintf("recac show %s", completedSessions[0].Name),
			Reason:  fmt.Sprintf("Review the work of your last completed session '%s'.", completedSessions[0].Name),
		})
		suggestions = append(suggestions, Suggestion{
			Command: "recac prune",
			Reason:  "You have completed sessions. Clean them up with the prune command.",
		})
	}

	// --- Default/General Suggestions ---
	if !isDirty && len(runningSessions) == 0 {
		suggestions = append(suggestions, Suggestion{
			Command: "recac start --goal \"<your goal>\"",
			Reason:  "No running sessions and a clean git status. A great time to start something new!",
		})
	}
	if len(sessions) > 5 {
		suggestions = append(suggestions, Suggestion{
			Command: "recac ls",
			Reason:  "You have many sessions. Use 'ls' to see them all, including archived ones.",
		})
	}

	// Remove duplicate suggestions (simple check based on command)
	seen := make(map[string]bool)
	var uniqueSuggestions []Suggestion
	for _, s := range suggestions {
		if !seen[s.Command] {
			uniqueSuggestions = append(uniqueSuggestions, s)
			seen[s.Command] = true
		}
	}

	return uniqueSuggestions
}

// getSessionsByStatus filters a slice of sessions by a given status.
func getSessionsByStatus(sessions []*runner.SessionState, status string) []*runner.SessionState {
	var filtered []*runner.SessionState
	for _, s := range sessions {
		if strings.EqualFold(s.Status, status) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}
