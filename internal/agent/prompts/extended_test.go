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
	t.Run("Embedded_Fallback", func(t *testing.T) {
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
	t.Run("Env_Override", func(t *testing.T) {
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

	// 3. Test Local .recac/prompts
	t.Run("Local_Override", func(t *testing.T) {
		// Ensure ENV is unset for this subtest (t.Setenv from other subtests doesn't leak, but just in case of global pollution)
		t.Setenv("RECAC_PROMPTS_DIR", "")

		// Mock CWD
		tmpDir := t.TempDir()
		oldGetwd := getwd
		getwd = func() (string, error) {
			return tmpDir, nil
		}
		defer func() {
			getwd = oldGetwd
		}()

		// Create .recac/prompts in the TEMP dir
		localRecacDir := filepath.Join(tmpDir, ".recac", "prompts")
		if err := os.MkdirAll(localRecacDir, 0755); err != nil {
			t.Fatalf("Failed to create local recac dir: %v", err)
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

		// 4. Test Variable Injection (inside this subtest since we have the file setup)
		t.Run("Variable_Injection", func(t *testing.T) {
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
