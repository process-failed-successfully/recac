package utils

import (
	"strings"
	"testing"
)

func TestGenerateDiff(t *testing.T) {
	original := "line1\nline2\nline3"
	improved := "line1\nline2 changed\nline3"

	diff, err := GenerateDiff("test.txt", original, improved)
	if err != nil {
		t.Fatalf("GenerateDiff failed: %v", err)
	}

	if !strings.Contains(diff, "--- test.txt") {
		t.Errorf("Diff missing original file header")
	}
	if !strings.Contains(diff, "+++ test.txt (improved)") {
		t.Errorf("Diff missing improved file header")
	}
	if !strings.Contains(diff, "-line2") {
		t.Errorf("Diff missing removal")
	}
	if !strings.Contains(diff, "+line2 changed") {
		t.Errorf("Diff missing addition")
	}
}

func TestGenerateDiff_NoChanges(t *testing.T) {
	original := "line1"
	improved := "line1"

	diff, err := GenerateDiff("test.txt", original, improved)
	if err != nil {
		t.Fatalf("GenerateDiff failed: %v", err)
	}

	if diff != "No changes.\n" {
		t.Errorf("Expected 'No changes.\n', got '%s'", diff)
	}
}
