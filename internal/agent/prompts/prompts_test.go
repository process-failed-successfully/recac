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
	list, err := ListPrompts()
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}

	// Check for known prompts
	found := false
	for _, p := range list {
		if p == Planner {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected ListPrompts to contain %q", Planner)
	}
}

func TestGetPrompt_LocalOverride(t *testing.T) {
	// Need to fake CWD or create .recac/prompts in CWD
	// Changing CWD in test is risky.
	// We can create a temp dir, chdir to it, run test, defer restore.

	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// Create .recac/prompts
	localPromptsDir := filepath.Join(tmpDir, ".recac", "prompts")
	if err := os.MkdirAll(localPromptsDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}

	promptName := "initializer"
	overrideContent := "Local override: {project_name}"
	path := filepath.Join(localPromptsDir, promptName+".md")
	if err := os.WriteFile(path, []byte(overrideContent), 0644); err != nil {
		t.Fatalf("failed to write override file: %v", err)
	}

	// Ensure ENV is not set (it might be set by other tests running in parallel if we weren't careful, but T.Setenv undoes it)
	// But check anyway? No, T.Setenv is safe.

	vars := map[string]string{"project_name": "MyProject"}
	got, err := GetPrompt(promptName, vars)
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}

	expected := "Local override: MyProject"
	if got != expected {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}

func TestGetPrompt_GlobalOverride(t *testing.T) {
	// Mocking os.UserHomeDir is hard without wrapper.
	// But we can set HOME env var which UserHomeDir respects on unix.

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	globalPromptsDir := filepath.Join(tmpHome, ".recac", "prompts")
	if err := os.MkdirAll(globalPromptsDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}

	promptName := "qa_agent"
	overrideContent := "Global override"
	path := filepath.Join(globalPromptsDir, promptName+".md")
	if err := os.WriteFile(path, []byte(overrideContent), 0644); err != nil {
		t.Fatalf("failed to write override file: %v", err)
	}

	// Ensure CWD does not have override
	// We are in original CWD or some temp dir from parallel tests?
	// Tests run in package dir by default.
	// We assume no .recac/prompts here.

	got, err := GetPrompt(promptName, nil)
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}

	if got != "Global override" {
		t.Errorf("Expected 'Global override', got %q", got)
	}
}

func TestGetPrompt_NotFound(t *testing.T) {
	_, err := GetPrompt("non_existent_prompt_12345", nil)
	if err == nil {
		t.Error("Expected error for non-existent prompt, got nil")
	}
}
