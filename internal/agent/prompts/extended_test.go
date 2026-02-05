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

	// 3. Test Local .recac/prompts (Mocked Getwd)

	// Unset env via Setenv
	t.Setenv("RECAC_PROMPTS_DIR", "")

	// Create mock CWD
	mockCwd := t.TempDir()

	// Create .recac/prompts structure
	localRecacDir := filepath.Join(mockCwd, ".recac", "prompts")
	if err := os.MkdirAll(localRecacDir, 0755); err != nil {
		t.Fatalf("Failed to create local .recac dir: %v", err)
	}

	localContent := "Local Override"
	err = os.WriteFile(filepath.Join(localRecacDir, promptName+".md"), []byte(localContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write local override: %v", err)
	}

	// Mock getwd
	originalGetwd := getwd
	defer func() { getwd = originalGetwd }()
	getwd = func() (string, error) {
		return mockCwd, nil
	}

	content, err = GetPrompt(promptName, nil)
	if err != nil {
		t.Fatalf("GetPrompt failed with local override: %v", err)
	}
	if content != localContent {
		t.Errorf("GetPrompt returned %q, want %q", content, localContent)
	}

	// 4. Test Global ~/.recac/prompts (Mocked UserHomeDir)

	// Create mock Home
	mockHome := t.TempDir()

	// Create ~/.recac/prompts structure
	globalRecacDir := filepath.Join(mockHome, ".recac", "prompts")
	if err := os.MkdirAll(globalRecacDir, 0755); err != nil {
		t.Fatalf("Failed to create global .recac dir: %v", err)
	}

	globalContent := "Global Override"
	err = os.WriteFile(filepath.Join(globalRecacDir, promptName+".md"), []byte(globalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write global override: %v", err)
	}

	// Mock userHomeDir
	originalUserHomeDir := userHomeDir
	defer func() { userHomeDir = originalUserHomeDir }()
	userHomeDir = func() (string, error) {
		return mockHome, nil
	}

	// Ensure local override is NOT present (mockCwd for this test)
	// We can reuse getwd mock, but let's make it point to a different empty dir or just ensure promptName is not there.
	// Actually, getwd is still mocked to return mockCwd which contains "Local Override".
	// Precedence rule: Local > Global.
	// So if we call GetPrompt now, it should return Local Override.

	content, err = GetPrompt(promptName, nil)
	if err != nil {
		t.Fatalf("GetPrompt failed with mixed overrides: %v", err)
	}
	// Expect Local because it has higher precedence than Global
	if content != localContent {
		t.Errorf("GetPrompt returned %q, want %q (Local precedence)", content, localContent)
	}

	// Now remove local override to test Global fallback
	if err := os.Remove(filepath.Join(localRecacDir, promptName+".md")); err != nil {
		t.Fatalf("Failed to remove local override: %v", err)
	}

	content, err = GetPrompt(promptName, nil)
	if err != nil {
		t.Fatalf("GetPrompt failed with global override: %v", err)
	}
	if content != globalContent {
		t.Errorf("GetPrompt returned %q, want %q (Global fallback)", content, globalContent)
	}

	// 5. Test Variable Injection
	// We can use the global file we just created.

	varContent := "Hello {name}"
	err = os.WriteFile(filepath.Join(globalRecacDir, promptName+".md"), []byte(varContent), 0644)
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
