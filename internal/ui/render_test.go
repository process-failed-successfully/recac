package ui

import (
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	input := "# Hello"
	output := RenderMarkdown(input, 80)
	// Glamour usually adds ANSI codes. We check if output is not empty and contains "Hello"
	if len(output) == 0 {
		t.Error("RenderMarkdown returned empty string")
	}
	if !strings.Contains(output, "Hello") {
		t.Error("RenderMarkdown output missing content")
	}
}

func TestGenerateLogo(t *testing.T) {
	logo := GenerateLogo()
	if len(logo) == 0 {
		t.Error("GenerateLogo returned empty string")
	}
	// Check for cached behavior (coverage)
	logo2 := GenerateLogo()
	if logo != logo2 {
		t.Error("GenerateLogo should return cached value")
	}
}
