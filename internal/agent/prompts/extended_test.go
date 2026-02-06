package prompts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPrompt_Overrides(t *testing.T) {
	promptName := "test_prompt"
	overrideContent := "Override Template"

	// Save original functions and restore after test
	origGetwd := getwd
	origUserHomeDir := userHomeDir
	defer func() {
		getwd = origGetwd
		userHomeDir = origUserHomeDir
	}()

	// 1. Test RECAC_PROMPTS_DIR (Env)
	t.Run("EnvOverride", func(t *testing.T) {
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

	// 2. Test Local .recac/prompts (Mocked CWD)
	t.Run("LocalOverride", func(t *testing.T) {
		// Ensure Env is cleared for this subtest
		t.Setenv("RECAC_PROMPTS_DIR", "")

		tmpCwd := t.TempDir()

		// Mock getwd
		getwd = func() (string, error) {
			return tmpCwd, nil
		}

		localRecacDir := filepath.Join(tmpCwd, ".recac", "prompts")
		if err := os.MkdirAll(localRecacDir, 0755); err != nil {
			t.Fatalf("Failed to create local dir: %v", err)
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
	})

	// 3. Test Global ~/.recac/prompts (Mocked Home)
	t.Run("GlobalOverride", func(t *testing.T) {
		t.Setenv("RECAC_PROMPTS_DIR", "")

		// Reset getwd to avoid local match from previous step (though temp dirs are unique, it's safer)
		// We mock getwd to return an empty dir so it falls through
		getwd = func() (string, error) {
			return t.TempDir(), nil
		}

		tmpHome := t.TempDir()

		// Mock userHomeDir
		userHomeDir = func() (string, error) {
			return tmpHome, nil
		}

		globalRecacDir := filepath.Join(tmpHome, ".recac", "prompts")
		if err := os.MkdirAll(globalRecacDir, 0755); err != nil {
			t.Fatalf("Failed to create global dir: %v", err)
		}

		globalContent := "Global Override"
		err := os.WriteFile(filepath.Join(globalRecacDir, promptName+".md"), []byte(globalContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write global override: %v", err)
		}

		content, err := GetPrompt(promptName, nil)
		if err != nil {
			t.Fatalf("GetPrompt failed with global override: %v", err)
		}
		if content != globalContent {
			t.Errorf("GetPrompt returned %q, want %q", content, globalContent)
		}
	})

	// 4. Test Variable Injection
	t.Run("VariableInjection", func(t *testing.T) {
		t.Setenv("RECAC_PROMPTS_DIR", "")
		tmpCwd := t.TempDir()
		getwd = func() (string, error) { return tmpCwd, nil }

		localRecacDir := filepath.Join(tmpCwd, ".recac", "prompts")
		os.MkdirAll(localRecacDir, 0755)

		varContent := "Hello {name}"
		os.WriteFile(filepath.Join(localRecacDir, promptName+".md"), []byte(varContent), 0644)

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
