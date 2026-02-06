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

	// Mock getwd and userHomeDir
	originalGetwd := getwd
	originalUserHomeDir := userHomeDir

	// Create a temp dir to serve as root for both CWD and Home
	tmpRoot := t.TempDir()

	// Define mocked functions
	mockedGetwd := func() (string, error) {
		return tmpRoot, nil
	}
	mockedUserHomeDir := func() (string, error) {
		return tmpRoot, nil // Use same root for simplicity, or specific subdir
	}

	// Apply mocks
	getwd = mockedGetwd
	userHomeDir = mockedUserHomeDir

	// Cleanup
	t.Cleanup(func() {
		getwd = originalGetwd
		userHomeDir = originalUserHomeDir
	})

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
	// We use a subdir for this to be distinct
	envDir := filepath.Join(tmpRoot, "env_prompts")
	os.MkdirAll(envDir, 0755)

	err = os.WriteFile(filepath.Join(envDir, promptName+".md"), []byte(overrideContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write override file: %v", err)
	}

	t.Setenv("RECAC_PROMPTS_DIR", envDir)

	content, err := GetPrompt(promptName, nil)
	if err != nil {
		t.Fatalf("GetPrompt failed with override: %v", err)
	}
	if content != overrideContent {
		t.Errorf("GetPrompt returned %q, want %q", content, overrideContent)
	}

	// 3. Test Local .recac/prompts
	// Disable Env override
	t.Setenv("RECAC_PROMPTS_DIR", "")

	// Create .recac/prompts in Mocked CWD (tmpRoot)
	localRecacDir := filepath.Join(tmpRoot, ".recac", "prompts")
	os.MkdirAll(localRecacDir, 0755)

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
