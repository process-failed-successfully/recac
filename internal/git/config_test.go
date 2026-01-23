package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClient_ConfigGlobal(t *testing.T) {
	// Create a temporary directory for the home directory/global config
	tmpDir := t.TempDir()

	// Create a dummy global config file
	configFile := filepath.Join(tmpDir, ".gitconfig")
	if err := os.WriteFile(configFile, []byte("[user]\n\tname = original\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Tell git to use this file as global config
	t.Setenv("GIT_CONFIG_GLOBAL", configFile)

	c := NewClient()

	// Test ConfigGlobal (set new value)
	if err := c.ConfigGlobal("user.email", "global@example.com"); err != nil {
		t.Errorf("ConfigGlobal failed: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if !strings.Contains(string(content), "email = global@example.com") {
		t.Errorf("ConfigGlobal did not update file. Content:\n%s", string(content))
	}

	// Test ConfigGlobal (overwrite existing)
	if err := c.ConfigGlobal("user.name", "updated"); err != nil {
		t.Errorf("ConfigGlobal failed: %v", err)
	}

	content, _ = os.ReadFile(configFile)
	if !strings.Contains(string(content), "name = updated") {
		t.Errorf("ConfigGlobal did not update user.name. Content:\n%s", string(content))
	}
}

func TestClient_ConfigAddGlobal(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, ".gitconfig")
	// Start empty
	if err := os.WriteFile(configFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("GIT_CONFIG_GLOBAL", configFile)

	c := NewClient()

	// Add multiple values for same key (e.g. core.excludesfile technically can have multiple?
	// Or maybe something like remote.origin.fetch)
	// Let's use a custom key for safety: my.list

	if err := c.ConfigAddGlobal("my.list", "item1"); err != nil {
		t.Errorf("ConfigAddGlobal failed: %v", err)
	}
	if err := c.ConfigAddGlobal("my.list", "item2"); err != nil {
		t.Errorf("ConfigAddGlobal failed: %v", err)
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "item1") || !strings.Contains(s, "item2") {
		t.Errorf("ConfigAddGlobal did not add both items. Content:\n%s", s)
	}
}
