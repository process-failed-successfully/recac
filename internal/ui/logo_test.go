package ui

import (
	"strings"
	"testing"
)

func TestGenerateLogo(t *testing.T) {
	logo := GenerateLogo()
	if logo == "" {
		t.Error("Logo should not be empty")
	}
	if !strings.Contains(logo, "RECAC") && !strings.Contains(logo, "____") {
		// ASCII art might not have clear text depending on style, but check for known parts
		// The ASCII art has "____", "|", etc.
		if !strings.Contains(logo, "|") {
			t.Error("Logo should contain ASCII characters")
		}
	}

	// Verify caching
	logo2 := GenerateLogo()
	if logo != logo2 {
		t.Error("Logo should be cached and identical")
	}
}
