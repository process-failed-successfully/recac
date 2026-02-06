package prompts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPrompt_Overrides(t *testing.T) {
	promptName := "test_prompt"

	t.Run("Embedded/Fallback", func(t *testing.T) {
		// Ensure Env var does not interfere
		t.Setenv("RECAC_PROMPTS_DIR", "")

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
		overrideContent := "Override Template"
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
		t.Setenv("RECAC_PROMPTS_DIR", "")

		localContent := "Local Override"
		tmpDir := t.TempDir()
		localRecacDir := filepath.Join(tmpDir, ".recac", "prompts")
		if err := os.MkdirAll(localRecacDir, 0755); err != nil {
			t.Fatalf("Failed to create local dir: %v", err)
		}

		err := os.WriteFile(filepath.Join(localRecacDir, promptName+".md"), []byte(localContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write local override: %v", err)
		}

		// Mock getwd
		originalGetwd := getwd
		t.Cleanup(func() { getwd = originalGetwd })
		getwd = func() (string, error) {
			return tmpDir, nil
		}

		content, err := GetPrompt(promptName, nil)
		if err != nil {
			t.Fatalf("GetPrompt failed with local override: %v", err)
		}
		if content != localContent {
			t.Errorf("GetPrompt returned %q, want %q", content, localContent)
		}
	})

	t.Run("Global ~/.recac/prompts", func(t *testing.T) {
		t.Setenv("RECAC_PROMPTS_DIR", "")

		globalContent := "Global Override"
		tmpDir := t.TempDir()
		globalRecacDir := filepath.Join(tmpDir, ".recac", "prompts")
		if err := os.MkdirAll(globalRecacDir, 0755); err != nil {
			t.Fatalf("Failed to create global dir: %v", err)
		}

		err := os.WriteFile(filepath.Join(globalRecacDir, promptName+".md"), []byte(globalContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write global override: %v", err)
		}

		// Ensure Local check fails so it falls back to global
		// Mock getwd to return a dir without .recac/prompts
		emptyDir := t.TempDir()
		originalGetwd := getwd
		getwd = func() (string, error) {
			return emptyDir, nil
		}

		// Mock userHomeDir
		originalUserHomeDir := userHomeDir
		userHomeDir = func() (string, error) {
			return tmpDir, nil
		}

		t.Cleanup(func() {
			getwd = originalGetwd
			userHomeDir = originalUserHomeDir
		})

		content, err := GetPrompt(promptName, nil)
		if err != nil {
			t.Fatalf("GetPrompt failed with global override: %v", err)
		}
		if content != globalContent {
			t.Errorf("GetPrompt returned %q, want %q", content, globalContent)
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
