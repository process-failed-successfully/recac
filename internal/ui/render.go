package ui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

var (
	// Keep existing styles if needed, but glamour handles most
	interactiveRenderer *glamour.TermRenderer
)

func init() {
	var err error
	interactiveRenderer, err = glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(80), // Reasonable default, view updates might override
	)
	if err != nil {
		// Fallback setup if needed, but NewTermRenderer rarely fails with defaults
	}
}

// RenderMarkdown parses markdown and returns an ANSI string using Glamour
func RenderMarkdown(text string, width int) string {
	// Re-init renderer if width changed significantly or just create new one (it's somewhat expensive though)
	// Ideally we keep one, but word-wrap needs dynamic width.
	// For basic char/TUI, creating one is 'okay', or we can set it.

	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return text // Fallback to raw text
	}

	out, err := r.Render(text)
	if err != nil {
		return text
	}

	// Glamour adds a newline at the end usually, trim it for chat bubbles
	return strings.TrimRight(out, "\n")
}
