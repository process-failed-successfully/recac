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

func TestGetPrompt_Local(t *testing.T) {
	// Create temp dir to simulate CWD
	tmpDir := t.TempDir()

	// Setup .recac/prompts
	promptsDir := filepath.Join(tmpDir, ".recac", "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write prompt
	promptName := "local_prompt"
	overrideContent := "Local prompt for {var}."
	path := filepath.Join(promptsDir, promptName+".md")
	if err := os.WriteFile(path, []byte(overrideContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Chdir
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Test
	got, err := GetPrompt(promptName, map[string]string{"var": "test"})
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}
	if got != "Local prompt for test." {
		t.Errorf("Expected local prompt, got %q", got)
	}
}

func TestGetPrompt_Global(t *testing.T) {
	// Create temp dir to simulate HOME
	tmpDir := t.TempDir()

	// Setup ~/.recac/prompts
	promptsDir := filepath.Join(tmpDir, ".recac", "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write prompt
	promptName := "global_prompt"
	overrideContent := "Global prompt for {var}."
	path := filepath.Join(promptsDir, promptName+".md")
	if err := os.WriteFile(path, []byte(overrideContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Set HOME
	t.Setenv("HOME", tmpDir)
	// Also USERPROFILE for Windows if running there, but linux container uses HOME.

	// Ensure CWD doesn't have it (we are in repo root or some test dir)

	// Test
	got, err := GetPrompt(promptName, map[string]string{"var": "test"})
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}
	if got != "Global prompt for test." {
		t.Errorf("Expected global prompt, got %q", got)
	}
}

func TestListPrompts(t *testing.T) {
	prompts, err := ListPrompts()
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}
	if len(prompts) == 0 {
		t.Error("Expected prompts list to be non-empty")
	}

	// Check for known prompt
	found := false
	for _, p := range prompts {
		if p == Planner {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'planner' in prompts list, got %v", prompts)
	}
}

func TestGetPrompt_Missing(t *testing.T) {
	_, err := GetPrompt("non_existent_prompt_12345", nil)
	if err == nil {
		t.Error("Expected error for missing prompt, got nil")
	}
}
