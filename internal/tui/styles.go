package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}

	// Status Colors
	statusRunning = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	statusDone    = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	statusFailed  = lipgloss.AdaptiveColor{Light: "#F25D94", Dark: "#FF7575"}
	statusPaused  = lipgloss.AdaptiveColor{Light: "#FAD85D", Dark: "#FFD93D"}

	// List Items
	localListItem = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(lipgloss.Color("241")) // Gray

	localSelectedListItem = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(highlight).
				Bold(true)

	// List Header
	localListHeader = lipgloss.NewStyle().
			Bold(true).
			PaddingLeft(2).
			PaddingBottom(1).
			PaddingTop(1).
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("235"))

	// Details View
	localDetailsStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingRight(2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle)

	localStatusStyle = lipgloss.NewStyle().
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1)
)

func statusColor(status string) lipgloss.TerminalColor {
	switch status {
	case "running":
		return statusRunning
	case "completed":
		return statusDone
	case "error", "failed":
		return statusFailed
	case "paused", "stopped":
		return statusPaused
	default:
		return subtle
	}
}
