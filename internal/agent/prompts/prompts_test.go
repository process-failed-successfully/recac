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
		t.Error("Expected prompts list to be non-empty")
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
		t.Error("Expected 'planner' in prompts list")
	}
}

func TestGetPrompt_Priority(t *testing.T) {
	// 2. Local .recac/prompts
	// We need to switch CWD
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpLocal := t.TempDir()

	// Setup .recac/prompts structure
	localPromptsDir := filepath.Join(tmpLocal, ".recac", "prompts")
	if err := os.MkdirAll(localPromptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localPromptsDir, "test_local.md"), []byte("Local Content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change CWD
	if err := os.Chdir(tmpLocal); err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Restore CWD
		_ = os.Chdir(cwd)
	}()

	content, err := GetPrompt("test_local", nil)
	if err != nil {
		t.Fatalf("GetPrompt failed for local: %v", err)
	}
	if content != "Local Content" {
		t.Errorf("Expected 'Local Content', got %q", content)
	}

	// 3. Global ~/.recac/prompts
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome) // Mock Home
	t.Setenv("USERPROFILE", tmpHome) // For Windows

	globalPromptsDir := filepath.Join(tmpHome, ".recac", "prompts")
	if err := os.MkdirAll(globalPromptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalPromptsDir, "test_global.md"), []byte("Global Content"), 0644); err != nil {
		t.Fatal(err)
	}

	content, err = GetPrompt("test_global", nil)
	if err != nil {
		t.Fatalf("GetPrompt failed for global: %v", err)
	}
	if content != "Global Content" {
		t.Errorf("Expected 'Global Content', got %q", content)
	}
}

func TestGetPrompt_NotFound(t *testing.T) {
	_, err := GetPrompt("non_existent_prompt", nil)
	if err == nil {
		t.Error("Expected error for non-existent prompt, got nil")
	}
}
