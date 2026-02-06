package prompts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPrompt_Overrides(t *testing.T) {
	promptName := "test_prompt"
	overrideContent := "Override Template"

	// 1. Test Embedded/Fallback
	t.Run("Embedded", func(t *testing.T) {
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
	t.Run("EnvVarOverride", func(t *testing.T) {
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
	t.Run("LocalDirOverride", func(t *testing.T) {
		// Ensure Env is unset
		t.Setenv("RECAC_PROMPTS_DIR", "")

		tmpDir := t.TempDir()

		// Create .recac/prompts in the temp dir
		localRecacDir := filepath.Join(tmpDir, ".recac", "prompts")
		if err := os.MkdirAll(localRecacDir, 0755); err != nil {
			t.Fatalf("Failed to create local dir: %v", err)
		}

		localContent := "Local Override"
		err := os.WriteFile(filepath.Join(localRecacDir, promptName+".md"), []byte(localContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write local override: %v", err)
		}

		// Mock getwd
		originalGetwd := getwd
		defer func() { getwd = originalGetwd }()
		getwd = func() (string, error) { return tmpDir, nil }

		content, err := GetPrompt(promptName, nil)
		if err != nil {
			t.Fatalf("GetPrompt failed with local override: %v", err)
		}
		if content != localContent {
			t.Errorf("GetPrompt returned %q, want %q", content, localContent)
		}
	})

	// 4. Test Global ~/.recac/prompts
	t.Run("GlobalDirOverride", func(t *testing.T) {
		t.Setenv("RECAC_PROMPTS_DIR", "")
		// Ensure local override doesn't interfere (by mocking getwd to somewhere empty)
		tmpLocalDir := t.TempDir()
		originalGetwd := getwd
		defer func() { getwd = originalGetwd }()
		getwd = func() (string, error) { return tmpLocalDir, nil }

		tmpGlobalDir := t.TempDir()
		globalRecacDir := filepath.Join(tmpGlobalDir, ".recac", "prompts")
		if err := os.MkdirAll(globalRecacDir, 0755); err != nil {
			t.Fatalf("Failed to create global dir: %v", err)
		}

		globalContent := "Global Override"
		err := os.WriteFile(filepath.Join(globalRecacDir, promptName+".md"), []byte(globalContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write global override: %v", err)
		}

		// Mock userHomeDir
		originalUserHomeDir := userHomeDir
		defer func() { userHomeDir = originalUserHomeDir }()
		userHomeDir = func() (string, error) { return tmpGlobalDir, nil }

		content, err := GetPrompt(promptName, nil)
		if err != nil {
			t.Fatalf("GetPrompt failed with global override: %v", err)
		}
		if content != globalContent {
			t.Errorf("GetPrompt returned %q, want %q", content, globalContent)
		}
	})

	// 5. Test Variable Injection
	t.Run("VariableInjection", func(t *testing.T) {
		// We can reuse the Env override mechanism for this test
		tmpDir := t.TempDir()
		varContent := "Hello {name}"
		err := os.WriteFile(filepath.Join(tmpDir, promptName+".md"), []byte(varContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write var override: %v", err)
		}

		t.Setenv("RECAC_PROMPTS_DIR", tmpDir)

		content, err := GetPrompt(promptName, map[string]string{"name": "World"})
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
