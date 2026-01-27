package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListPrompts(t *testing.T) {
	prompts, err := ListPrompts()
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}

	if len(prompts) == 0 {
		t.Error("Expected at least one prompt")
	}

	found := false
	for _, p := range prompts {
		if p == Planner {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected prompt %q to be in the list", Planner)
	}
}

func TestGetPrompt_LocalOverride(t *testing.T) {
	tempDir := t.TempDir()
	promptsDir := filepath.Join(tempDir, ".recac", "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatalf("Failed to create prompts dir: %v", err)
	}

	promptName := "local_test_prompt"
	promptContent := "This is a local override for {test_var}."
	promptPath := filepath.Join(promptsDir, promptName+".md")
	if err := os.WriteFile(promptPath, []byte(promptContent), 0644); err != nil {
		t.Fatalf("Failed to write prompt file: %v", err)
	}

    // Change WD to the temp dir
    t.Chdir(tempDir)

	vars := map[string]string{
		"test_var": "UNIT TEST",
	}
	got, err := GetPrompt(promptName, vars)
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}

	expected := "This is a local override for UNIT TEST."
	if got != expected {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}

func TestGetPrompt_GlobalOverride(t *testing.T) {
	tempHome := t.TempDir()
	promptsDir := filepath.Join(tempHome, ".recac", "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatalf("Failed to create prompts dir: %v", err)
	}

	promptName := "global_test_prompt"
	promptContent := "This is a global override for {test_var}."
	promptPath := filepath.Join(promptsDir, promptName+".md")
	if err := os.WriteFile(promptPath, []byte(promptContent), 0644); err != nil {
		t.Fatalf("Failed to write prompt file: %v", err)
	}

	// Mock HOME
	t.Setenv("HOME", tempHome)
    t.Setenv("USERPROFILE", tempHome)

    // Ensure RECAC_PROMPTS_DIR is unset
    t.Setenv("RECAC_PROMPTS_DIR", "")

	vars := map[string]string{
		"test_var": "GLOBAL TEST",
	}
	got, err := GetPrompt(promptName, vars)
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}

	expected := "This is a global override for GLOBAL TEST."
	if got != expected {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}

func TestGetPrompt_NotFound(t *testing.T) {
    // Ensure no overrides interfere
    t.Setenv("RECAC_PROMPTS_DIR", "")
    t.Setenv("HOME", t.TempDir()) // Empty home

    // Using a random name
    _, err := GetPrompt("non_existent_prompt_12345", nil)
    if err == nil {
        t.Error("Expected error for non-existent prompt, got nil")
    }

    if !strings.Contains(err.Error(), "failed to read prompt template") {
        t.Errorf("Expected error message to contain 'failed to read prompt template', got: %v", err)
    }
}
