package main

import (
	"fmt"
	"os"

	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var playbackCmd = &cobra.Command{
	Use:   "playback [session-name]",
	Short: "Interactive session log playback",
	Long:  `Replay and analyze session logs interactively using a TUI.
Allows filtering, searching, and detailed inspection of agent actions and tool outputs.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		// Get log path
		logPath, err := sm.GetSessionLogs(sessionName)
		if err != nil {
			return fmt.Errorf("failed to get logs for session '%s': %w", sessionName, err)
		}

		// Read log file
		content, err := os.ReadFile(logPath)
		if err != nil {
			return fmt.Errorf("failed to read log file: %w", err)
		}

		// Parse logs
		entries, err := ui.ParseLogLines(content)
		if err != nil {
			return fmt.Errorf("failed to parse logs: %w", err)
		}

		if len(entries) == 0 {
			fmt.Println("No log entries found.")
			return nil
		}

		// Start TUI
		m := ui.NewPlaybackModel(entries)
		p := tea.NewProgram(m, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running playback TUI: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(playbackCmd)
}
