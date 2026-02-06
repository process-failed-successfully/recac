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

	// 1. Test Embedded/Fallback (simulated by failure of others)
	// We can't easily add to embed.FS at runtime, but we can test that GetPrompt returns error for non-existent if no override exists.
	// Or we can rely on existing templates.
	// Let's rely on existing "planner" template if it exists, or handle error.

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

	// 2. Test RECAC_PROMPTS_DIR
	tmpDir := t.TempDir()

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

	// 3. Test Local .recac/prompts
	t.Setenv("RECAC_PROMPTS_DIR", "")

	// Mock getwd to point to a temp dir
	cwdDir := t.TempDir()

	// Backup original getwd
	originalGetwd := getwd
	defer func() { getwd = originalGetwd }()

	getwd = func() (string, error) {
		return cwdDir, nil
	}

	localRecacDir := filepath.Join(cwdDir, ".recac", "prompts")
	if err := os.MkdirAll(localRecacDir, 0755); err != nil {
		t.Fatalf("Failed to create local recac dir: %v", err)
	}

	localContent := "Local Override"
	err = os.WriteFile(filepath.Join(localRecacDir, promptName+".md"), []byte(localContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write local override: %v", err)
	}

	content, err = GetPrompt(promptName, nil)
	if err != nil {
		t.Fatalf("GetPrompt failed with local override: %v", err)
	}
	if content != localContent {
		t.Errorf("GetPrompt returned %q, want %q", content, localContent)
	}

	// 4. Test Variable Injection
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

	// 5. Test Global ~/.recac/prompts
	// Ensure local override doesn't exist for this test part
	globalPromptName := "global_test_prompt"
	globalContent := "Global Override"

	// Setup global dir in temp dir
	homeDir := t.TempDir()
	globalRecacDir := filepath.Join(homeDir, ".recac", "prompts")
	if err := os.MkdirAll(globalRecacDir, 0755); err != nil {
		t.Fatalf("Failed to create global recac dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalRecacDir, globalPromptName+".md"), []byte(globalContent), 0644); err != nil {
		t.Fatalf("Failed to write global override: %v", err)
	}

	// Mock userHomeDir
	originalUserHomeDir := userHomeDir
	defer func() { userHomeDir = originalUserHomeDir }()
	userHomeDir = func() (string, error) {
		return homeDir, nil
	}

	content, err = GetPrompt(globalPromptName, nil)
	if err != nil {
		t.Fatalf("GetPrompt failed with global override: %v", err)
	}
	if content != globalContent {
		t.Errorf("GetPrompt returned %q, want %q", content, globalContent)
	}
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
