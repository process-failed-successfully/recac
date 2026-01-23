package ui

import (
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	input := "**Bold**"
	output := RenderMarkdown(input, 80)
	// Glamour renders bold text with ANSI codes.
	// We check that input is transformed (at least checking length or content).
	if output == "" {
		t.Error("Rendered markdown should not be empty")
	}
	// Check for "Bold" content
	if !strings.Contains(output, "Bold") {
		t.Error("Rendered markdown should contain content")
	}
}
