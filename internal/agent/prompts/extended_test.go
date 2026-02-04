package prompts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPrompt_Overrides(t *testing.T) {
	promptName := "test_prompt"
	overrideContent := "Override Template"

	t.Run("Embedded/Fallback", func(t *testing.T) {
		// Check ListPrompts first
		prompts, err := ListPrompts()
		if err != nil {
			t.Fatalf("ListPrompts failed: %v", err)
		}
		if len(prompts) == 0 {
			t.Log("No embedded prompts found, skipping embedded test verification")
		} else {
			// Use the first available prompt
			pName := prompts[0]
			pContent, err := GetPrompt(pName, nil)
			if err != nil {
				t.Errorf("GetPrompt failed for embedded %s: %v", pName, err)
			}
			if len(pContent) == 0 {
				t.Error("Embedded prompt content is empty")
			}
		}
	})

	t.Run("RECAC_PROMPTS_DIR", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := os.WriteFile(filepath.Join(tmpDir, promptName+".md"), []byte(overrideContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write override file: %v", err)
		}

		t.Setenv("RECAC_PROMPTS_DIR", tmpDir)

		content, err := GetPrompt(promptName, nil)
		if err != nil {
			t.Fatalf("GetPrompt failed with override: %v", err)
		}
		if content != overrideContent {
			t.Errorf("GetPrompt returned %q, want %q", content, overrideContent)
		}
	})

	t.Run("Local .recac/prompts", func(t *testing.T) {
		// Ensure RECAC_PROMPTS_DIR is unset/empty
		t.Setenv("RECAC_PROMPTS_DIR", "")

		// Create temp dir and switch to it
		tmpDir := t.TempDir()

		// Save current WD and restore it after test
		oldWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current working directory: %v", err)
		}
		defer func() {
			if err := os.Chdir(oldWd); err != nil {
				t.Errorf("Failed to restore working directory: %v", err)
			}
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}

		// Create .recac/prompts in the temp dir (which is now CWD)
		localRecacDir := filepath.Join(tmpDir, ".recac", "prompts")
		if err := os.MkdirAll(localRecacDir, 0755); err != nil {
			t.Fatalf("Failed to create .recac/prompts: %v", err)
		}

		localContent := "Local Override"
		err = os.WriteFile(filepath.Join(localRecacDir, promptName+".md"), []byte(localContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write local override: %v", err)
		}

		content, err := GetPrompt(promptName, nil)
		if err != nil {
			t.Fatalf("GetPrompt failed with local override: %v", err)
		}
		if content != localContent {
			t.Errorf("GetPrompt returned %q, want %q", content, localContent)
		}

		// Test Variable Injection
		varContent := "Hello {name}"
		err = os.WriteFile(filepath.Join(localRecacDir, promptName+".md"), []byte(varContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write var override: %v", err)
		}

		content, err = GetPrompt(promptName, map[string]string{"name": "World"})
		if err != nil {
			t.Fatalf("GetPrompt failed with vars: %v", err)
		}
		if content != "Hello World" {
			t.Errorf("GetPrompt returned %q, want %q", content, "Hello World")
		}
	})
}

func TestListPrompts(t *testing.T) {
	prompts, err := ListPrompts()
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}

	// We expect at least some standard prompts
	expected := []string{"planner", "coding_agent", "initializer"}

	for _, exp := range expected {
		found := false
		for _, p := range prompts {
			if p == exp {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Warning: Expected prompt %s not found in list %v", exp, prompts)
		}
	}
}
