package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ASCII Art Logo
const asciiLogo = `
  ____  _____ _____ _    ____ 
 |  _ \| ____/ ____/ \  / ___|
 | |_) |  _|| |   / _ \| |    
 |  _ <| |__| |__/ ___ \ |___ 
 |_| \_\_____\____/_/   \_\____|
`

// GenerateLogo returns the gradient styled logo
func GenerateLogo() string {
	lines := strings.Split(strings.Trim(asciiLogo, "\n"), "\n")
	var coloredLines []string

	for i, line := range lines {
		var color string

		// Simple manual gradient approximation
		switch i {
		case 0:
			color = "#00BFFF" // Deep Sky Blue
		case 1:
			color = "#1E90FF" // Dodger Blue
		case 2:
			color = "#4169E1" // Royal Blue
		case 3:
			color = "#8A2BE2" // Blue Violet
		case 4:
			color = "#FF00FF" // Magenta
		default:
			color = "#FFF"
		}

		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true)
		coloredLines = append(coloredLines, style.Render(line))
	}

	return strings.Join(coloredLines, "\n")
}

// LogoContainerStyle container
var LogoContainerStyle = lipgloss.NewStyle().
	MarginLeft(2).
	MarginBottom(1).
	Padding(0, 1).
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("62")) // Purple-ish border
