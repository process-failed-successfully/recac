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
	names, err := ListPrompts()
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}
	if len(names) == 0 {
		t.Error("Expected prompts, got empty list")
	}

	found := false
	for _, n := range names {
		if n == Planner {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'planner' in prompts list")
	}
}

func TestGetPrompt_LocalOverride(t *testing.T) {
	// Create .recac/prompts in CWD
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Create temporary directory structure
	localDir := filepath.Join(cwd, ".recac", "prompts")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Join(cwd, ".recac"))

	promptName := "planner"
	overrideContent := "Local override {spec}"
	path := filepath.Join(localDir, promptName+".md")
	if err := os.WriteFile(path, []byte(overrideContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Ensure Env is cleared
	t.Setenv("RECAC_PROMPTS_DIR", "")

	got, err := GetPrompt(promptName, map[string]string{"spec": "FOO"})
	if err != nil {
		t.Fatal(err)
	}

	if got != "Local override FOO" {
		t.Errorf("Expected local override, got %q", got)
	}
}

func TestGetPrompt_Fallback(t *testing.T) {
	// Ensure no overrides
	t.Setenv("RECAC_PROMPTS_DIR", "")

	// We assume CWD doesn't have .recac/prompts (TestGetPrompt_LocalOverride cleans up)
	// We assume user home doesn't have it either (or we can't easily control it).
	// But we can check for an embedded one.

	got, err := GetPrompt(CodingAgent, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Error("Expected embedded prompt content")
	}
}
