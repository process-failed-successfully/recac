package main

import (
	"fmt"
	"time"

	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	focusDuration string
	focusBreak    string
	focusTask     string
)

var focusCmd = &cobra.Command{
	Use:   "focus",
	Short: "Start a focus timer (Pomodoro)",
	Long:  `Starts a TUI-based focus timer to help you stay productive. Default duration is 25 minutes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		duration, err := time.ParseDuration(focusDuration)
		if err != nil {
			return fmt.Errorf("invalid duration format: %w", err)
		}

		// Optional: We could also parse break duration if we implement break logic later
		// _, err = time.ParseDuration(focusBreak)
		// if err != nil {
		// 	return fmt.Errorf("invalid break duration format: %w", err)
		// }

		model := ui.NewFocusModel(duration, focusTask)
		p := tea.NewProgram(model, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running focus timer: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(focusCmd)
	focusCmd.Flags().StringVarP(&focusDuration, "duration", "d", "25m", "Duration of the focus session (e.g. 25m, 1h)")
	focusCmd.Flags().StringVarP(&focusBreak, "break", "b", "5m", "Duration of the break (e.g. 5m)")
	focusCmd.Flags().StringVarP(&focusTask, "task", "t", "", "Name of the task you are focusing on")
}
