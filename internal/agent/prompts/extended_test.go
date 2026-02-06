package prompts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPrompt_Overrides(t *testing.T) {
	// Mock Setup for package-level variables
	// We do this at the start to ensure restoration happens even if test fails midway
	origGetwd := getwd
	origUserHomeDir := userHomeDir
	t.Cleanup(func() {
		getwd = origGetwd
		userHomeDir = origUserHomeDir
	})

	// Setup
	promptName := "test_prompt"
	overrideContent := "Override Template"

	// 1. Test Embedded/Fallback
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

	// Clear env for next tests
	t.Setenv("RECAC_PROMPTS_DIR", "")

	// 3. Test Local .recac/prompts (Mocked)
	mockCwd := t.TempDir()

	// Create structure inside mockCwd: .recac/prompts
	localRecacDir := filepath.Join(mockCwd, ".recac", "prompts")
	if err := os.MkdirAll(localRecacDir, 0755); err != nil {
		t.Fatalf("Failed to create local .recac dir: %v", err)
	}

	localContent := "Local Override"
	if err := os.WriteFile(filepath.Join(localRecacDir, promptName+".md"), []byte(localContent), 0644); err != nil {
		t.Fatalf("Failed to write local override: %v", err)
	}

	// Mock getwd to return mockCwd
	getwd = func() (string, error) { return mockCwd, nil }

	content, err = GetPrompt(promptName, nil)
	if err != nil {
		t.Fatalf("GetPrompt failed with local override: %v", err)
	}
	if content != localContent {
		t.Errorf("GetPrompt returned %q, want %q", content, localContent)
	}

	// 4. Test Global ~/.recac/prompts (Mocked)
	mockHome := t.TempDir()
	globalRecacDir := filepath.Join(mockHome, ".recac", "prompts")
	if err := os.MkdirAll(globalRecacDir, 0755); err != nil {
		t.Fatalf("Failed to create global .recac dir: %v", err)
	}

	globalContent := "Global Override"
	if err := os.WriteFile(filepath.Join(globalRecacDir, promptName+".md"), []byte(globalContent), 0644); err != nil {
		t.Fatalf("Failed to write global override: %v", err)
	}

	// Mock userHomeDir to return mockHome
	userHomeDir = func() (string, error) { return mockHome, nil }

	// Point getwd to an empty dir so local check fails and we fall back to global
	emptyDir := t.TempDir()
	getwd = func() (string, error) { return emptyDir, nil }

	content, err = GetPrompt(promptName, nil)
	if err != nil {
		t.Fatalf("GetPrompt failed with global override: %v", err)
	}
	if content != globalContent {
		t.Errorf("GetPrompt returned %q, want %q", content, globalContent)
	}

	// 5. Test Variable Injection (using Global Override)
	varContent := "Hello {name}"
	if err := os.WriteFile(filepath.Join(globalRecacDir, promptName+".md"), []byte(varContent), 0644); err != nil {
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
