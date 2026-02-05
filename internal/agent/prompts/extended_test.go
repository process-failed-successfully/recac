package prompts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPrompt_Overrides(t *testing.T) {
	promptName := "test_prompt"

	// Save original functions and restore after test
	origGetwd := getwd
	origUserHomeDir := userHomeDir
	defer func() {
		getwd = origGetwd
		userHomeDir = origUserHomeDir
	}()

	t.Run("Embedded/Fallback", func(t *testing.T) {
		// Use an existing prompt name from ListPrompts
		prompts, err := ListPrompts()
		if err != nil {
			t.Fatalf("ListPrompts failed: %v", err)
		}
		if len(prompts) > 0 {
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
		t.Setenv("RECAC_PROMPTS_DIR", "") // Ensure env override is off

		localContent := "Local Override"
		tmpCwd := t.TempDir()

		// Create .recac/prompts structure
		promptsDir := filepath.Join(tmpCwd, ".recac", "prompts")
		if err := os.MkdirAll(promptsDir, 0755); err != nil {
			t.Fatalf("Failed to create .recac dir: %v", err)
		}

		err := os.WriteFile(filepath.Join(promptsDir, promptName+".md"), []byte(localContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write local override: %v", err)
		}

		// Mock getwd
		getwd = func() (string, error) {
			return tmpCwd, nil
		}
		// Restore is handled by outer defer, but safer to restore per subtest if needed.
		// Here we don't strictly need to restore immediately as subsequent tests set their own mocks or don't rely on it.
		// But for cleanliness:
		defer func() { getwd = origGetwd }()

		content, err := GetPrompt(promptName, nil)
		if err != nil {
			t.Fatalf("GetPrompt failed with local override: %v", err)
		}
		if content != localContent {
			t.Errorf("GetPrompt returned %q, want %q", content, localContent)
		}
	})

	t.Run("Global ~/.recac/prompts", func(t *testing.T) {
		t.Setenv("RECAC_PROMPTS_DIR", "") // Ensure env override is off

		// Mock getwd to return empty temp dir to ensure local check doesn't find anything
		emptyCwd := t.TempDir()
		getwd = func() (string, error) { return emptyCwd, nil }
		defer func() { getwd = origGetwd }()

		globalContent := "Global Override"
		tmpHome := t.TempDir()

		// Create .recac/prompts structure
		promptsDir := filepath.Join(tmpHome, ".recac", "prompts")
		if err := os.MkdirAll(promptsDir, 0755); err != nil {
			t.Fatalf("Failed to create .recac dir: %v", err)
		}

		err := os.WriteFile(filepath.Join(promptsDir, promptName+".md"), []byte(globalContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write global override: %v", err)
		}

		// Mock userHomeDir
		userHomeDir = func() (string, error) {
			return tmpHome, nil
		}
		defer func() { userHomeDir = origUserHomeDir }()

		content, err := GetPrompt(promptName, nil)
		if err != nil {
			t.Fatalf("GetPrompt failed with global override: %v", err)
		}
		if content != globalContent {
			t.Errorf("GetPrompt returned %q, want %q", content, globalContent)
		}
	})

	t.Run("Variable Injection", func(t *testing.T) {
		// Use Env override for simplicity
		tmpDir := t.TempDir()
		t.Setenv("RECAC_PROMPTS_DIR", tmpDir)

		varContent := "Hello {name}"
		err := os.WriteFile(filepath.Join(tmpDir, promptName+".md"), []byte(varContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write var override: %v", err)
		}

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
