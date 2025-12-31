package prompts

import (
	"strings"
	"testing"
)

func TestGetPrompt(t *testing.T) {
	// Test Planner Prompt
	vars := map[string]string{
		"spec": "Test Specification Content",
	}
	got, err := GetPrompt(Planner, vars)
	if err != nil {
		t.Fatalf("GetPrompt(Planner) failed: %v", err)
	}

	if !strings.Contains(got, "Lead Software Architect") {
		t.Errorf("Expected prompt to contain 'Lead Software Architect', got %q", got)
	}
	if !strings.Contains(got, "Test Specification Content") {
		t.Errorf("Expected prompt to contain 'Test Specification Content', got %q", got)
	}

	// Test Manager Review Prompt
	vars2 := map[string]string{
		"qa_report": "All tests passed!",
	}
	got2, err := GetPrompt(ManagerReview, vars2)
	if err != nil {
		t.Fatalf("GetPrompt(ManagerReview) failed: %v", err)
	}
	if !strings.Contains(got2, "PROJECT MANAGER") {
		t.Errorf("Expected prompt to contain 'PROJECT MANAGER', got %q", got2)
	}
	if !strings.Contains(got2, "All tests passed!") {
		t.Errorf("Expected prompt to contain 'All tests passed!', got %q", got2)
	}
}
