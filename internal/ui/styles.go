package ui

import "github.com/charmbracelet/lipgloss"

// This file centralizes the lipgloss styles used across the TUI.

var (
	// General Panes
	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")) // Purple-ish

	// Headers and Footers
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")).
			Background(lipgloss.Color("#7D56F4")). // Brand Color
			Bold(true).
			Padding(0, 1)

	interactiveTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFF")).
				Background(lipgloss.Color("#874BFD")).
				Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	// Log/Message Styles
	logInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")) // Light Gray
	logThoughtStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")) // Cyan/Teal for thoughts
	logErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")). // Red
			Bold(true)
	logSuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")). // Green
			Bold(true)

	// List Styles (for dashboard)
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")).
			Background(lipgloss.Color("63")). // Purple
			Padding(0, 1)

	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("170")) // Magenta
	paginationStyle = lipgloss.NewStyle().PaddingLeft(4)
	helpStyle       = lipgloss.NewStyle().PaddingLeft(4).PaddingBottom(1)
	quitTextStyle   = lipgloss.NewStyle().Margin(1, 0, 2, 4)

	// Detail View (for dashboard)
	detailTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("212")). // Light purple
				MarginBottom(1)

	detailTextStyle = lipgloss.NewStyle().
			MarginLeft(2)
)
