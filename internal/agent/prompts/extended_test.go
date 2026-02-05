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

	// 2. Test RECAC_PROMPTS_DIR
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

	// 3. Test Local .recac/prompts (Mocked CWD)
	t.Run("LocalOverride", func(t *testing.T) {
		// Ensure Env var is not set
		t.Setenv("RECAC_PROMPTS_DIR", "")

		// Setup mock CWD
		tmpCwd := t.TempDir()
		origGetwd := getwd
		getwd = func() (string, error) { return tmpCwd, nil }
		defer func() { getwd = origGetwd }()

		// Create .recac/prompts
		localRecacDir := filepath.Join(tmpCwd, ".recac", "prompts")
		if err := os.MkdirAll(localRecacDir, 0755); err != nil {
			t.Fatalf("Failed to create local dir: %v", err)
		}

		localContent := "Local Override"
		if err := os.WriteFile(filepath.Join(localRecacDir, promptName+".md"), []byte(localContent), 0644); err != nil {
			t.Fatalf("Failed to write local override: %v", err)
		}

		content, err := GetPrompt(promptName, nil)
		if err != nil {
			t.Fatalf("GetPrompt failed with local override: %v", err)
		}
		if content != localContent {
			t.Errorf("GetPrompt returned %q, want %q", content, localContent)
		}

		// 4. Test Variable Injection (using same local setup)
		varContent := "Hello {name}"
		if err := os.WriteFile(filepath.Join(localRecacDir, promptName+".md"), []byte(varContent), 0644); err != nil {
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

	// 5. Test Global ~/.recac/prompts (Mocked UserHome)
	t.Run("GlobalOverride", func(t *testing.T) {
		t.Setenv("RECAC_PROMPTS_DIR", "")

		// Setup mock Home
		tmpHome := t.TempDir()
		origUserHomeDir := userHomeDir
		userHomeDir = func() (string, error) { return tmpHome, nil }
		defer func() { userHomeDir = origUserHomeDir }()

		// Ensure CWD doesn't have it (using empty temp dir as CWD to be safe)
		tmpCwd := t.TempDir()
		origGetwd := getwd
		getwd = func() (string, error) { return tmpCwd, nil }
		defer func() { getwd = origGetwd }()

		// Create ~/.recac/prompts
		globalRecacDir := filepath.Join(tmpHome, ".recac", "prompts")
		if err := os.MkdirAll(globalRecacDir, 0755); err != nil {
			t.Fatalf("Failed to create global dir: %v", err)
		}

		globalContent := "Global Override"
		if err := os.WriteFile(filepath.Join(globalRecacDir, promptName+".md"), []byte(globalContent), 0644); err != nil {
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
