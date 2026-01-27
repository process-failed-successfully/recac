package prompts

import (
	"os"
	"path/filepath"
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

func TestGetPrompt_Override(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()

	// Set ENV
	t.Setenv("RECAC_PROMPTS_DIR", tmpDir)

	// Write override file
	promptName := "coding_agent"
	overrideContent := "This is an overridden prompt for {task_id}."
	path := filepath.Join(tmpDir, promptName+".md")
	if err := os.WriteFile(path, []byte(overrideContent), 0644); err != nil {
		t.Fatalf("failed to write override file: %v", err)
	}

	// Test
	vars := map[string]string{
		"task_id": "TASK-123",
	}
	got, err := GetPrompt(promptName, vars)
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}

	expected := "This is an overridden prompt for TASK-123."
	if got != expected {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}

func TestListPrompts(t *testing.T) {
	prompts, err := ListPrompts()
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}

	if len(prompts) == 0 {
		t.Errorf("Expected prompts to be returned, got 0")
	}

	// Check for a known prompt
	found := false
	for _, p := range prompts {
		if p == Planner {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected prompt %q to be in the list, got %v", Planner, prompts)
	}
}
