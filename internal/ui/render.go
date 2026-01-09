package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	codeBlockStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")). // Light text
			Background(lipgloss.Color("236")). // Dark background
			Padding(1, 2).
			Margin(1, 0)

	inlineCodeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")). // Blueish
			Background(lipgloss.Color("236"))

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true).
			MarginTop(1)

	boldStyle = lipgloss.NewStyle().Bold(true)
)

// RenderMarkdown provides basic markdown rendering for TUI
func RenderMarkdown(text string) string {
	// Split by code blocks to avoid formatting inside them
	parts := strings.Split(text, "```")
	var sb strings.Builder

	headerRe := regexp.MustCompile(`(?m)^#+\s+(.*)$`)
	boldRe := regexp.MustCompile(`\*\*(.*?)\*\*`)
	inlineCodeRe := regexp.MustCompile("`([^`]+)`")

	for i, part := range parts {
		if i%2 == 0 {
			// Normal text processing

			// Headers
			part = headerRe.ReplaceAllStringFunc(part, func(s string) string {
				content := strings.TrimLeft(s, "# ")
				return headerStyle.Render(content)
			})

			// Bold
			part = boldRe.ReplaceAllStringFunc(part, func(s string) string {
				match := boldRe.FindStringSubmatch(s)
				if len(match) > 1 {
					return boldStyle.Render(match[1])
				}
				return s
			})

			// Inline Code
			part = inlineCodeRe.ReplaceAllStringFunc(part, func(s string) string {
				match := inlineCodeRe.FindStringSubmatch(s)
				if len(match) > 1 {
					return inlineCodeStyle.Render(match[1])
				}
				return s
			})

			sb.WriteString(part)
		} else {
			// Code block content
			// Check if first line is lang identifier
			lines := strings.Split(strings.TrimSpace(part), "\n")
			if len(lines) > 0 {
				first := strings.TrimSpace(lines[0])
				// Simple heuristic: if first line is single word, it's likely lang
				if len(strings.Fields(first)) == 1 && first != "" {
					// Remove language line
					part = strings.Join(lines[1:], "\n")
				}
			}
			sb.WriteString(codeBlockStyle.Render(strings.TrimSpace(part)))
		}
	}

	return sb.String()
}
