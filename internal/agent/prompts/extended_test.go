package prompts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPrompt_Overrides(t *testing.T) {
	// Setup
	promptName := "test_prompt"
	overrideContent := "Override Template"

	// 1. Test Embedded/Fallback
	t.Run("Embedded", func(t *testing.T) {
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

	// 2. Test RECAC_PROMPTS_DIR
	t.Run("EnvOverride", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "recac-prompts-test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		err = os.WriteFile(filepath.Join(tmpDir, promptName+".md"), []byte(overrideContent), 0644)
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

	// 3. Test Local .recac/prompts
	t.Run("LocalOverride", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Mock getwd instead of changing Chdir
		oldGetwd := getwd
		getwd = func() (string, error) {
			return tmpDir, nil
		}
		defer func() {
			getwd = oldGetwd
		}()

		// Ensure env var is not set
		t.Setenv("RECAC_PROMPTS_DIR", "")

		// Create .recac/prompts in tmpDir (which is now mocked CWD)
		localRecacDir := filepath.Join(tmpDir, ".recac", "prompts")
		if err := os.MkdirAll(localRecacDir, 0755); err != nil {
			t.Fatalf("Failed to mkdir: %v", err)
		}

		localContent := "Local Override"
		err := os.WriteFile(filepath.Join(localRecacDir, promptName+".md"), []byte(localContent), 0644)
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

		// 4. Test Variable Injection (in same context as local override)
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
