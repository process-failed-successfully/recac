package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// setupTestConfig creates a temporary config file.
// It returns the path to the config file and a cleanup function.
func setupTestConfig(t *testing.T) (string, func()) {
	t.Helper()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
debug: true
timeout: 90s
agent_provider: "test_provider"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write dummy config file: %v", err)
	}

	cleanup := func() {
		viper.Reset()
	}

	return configPath, cleanup
}

func TestConfigViewCmd(t *testing.T) {
	configPath, cleanup := setupTestConfig(t)
	defer cleanup()

	// Execute the command with the --config flag
	output, err := executeCommand(rootCmd, "--config", configPath, "config", "view")
	if err != nil {
		t.Fatalf("execute command failed: %v", err)
	}

	// Check the output
	expectedSubstrings := []string{
		"debug: true",
		"timeout: 90s",
		"agent_provider: test_provider",
	}

	for _, s := range expectedSubstrings {
		if !strings.Contains(output, s) {
			t.Errorf("expected output to contain %q, but got %q", s, output)
		}
	}
}

func TestConfigGetCmd(t *testing.T) {
	configPath, cleanup := setupTestConfig(t)
	defer cleanup()

	// Test getting an existing key
	output, err := executeCommand(rootCmd, "--config", configPath, "config", "get", "debug")
	if err != nil {
		t.Fatalf("execute command failed: %v", err)
	}
	if got := strings.TrimSpace(output); got != "true" {
		t.Errorf("expected output to be 'true', but got %q", got)
	}

	// Test getting another key
	output, err = executeCommand(rootCmd, "--config", configPath, "config", "get", "timeout")
	if err != nil {
		t.Fatalf("execute command failed: %v", err)
	}
	if got := strings.TrimSpace(output); got != "90s" {
		t.Errorf("expected output to be '90s', but got %q", got)
	}

	// Test getting a non-existent key
	_, err = executeCommand(rootCmd, "--config", configPath, "config", "get", "nonexistent")
	if err == nil {
		t.Fatal("expected an error for non-existent key, but got none")
	}
	if !strings.Contains(err.Error(), "key not found") {
		t.Errorf("expected error message to contain 'key not found', but got %q", err.Error())
	}
}

func TestConfigSetCmd(t *testing.T) {
	configPath, cleanup := setupTestConfig(t)
	defer cleanup()

	// Set a new value
	_, err := executeCommand(rootCmd, "--config", configPath, "config", "set", "debug", "false")
	if err != nil {
		t.Fatalf("execute set command failed: %v", err)
	}

	// Verify the value was set
	output, err := executeCommand(rootCmd, "--config", configPath, "config", "get", "debug")
	if err != nil {
		t.Fatalf("execute get command failed: %v", err)
	}
	if got := strings.TrimSpace(output); got != "false" {
		t.Errorf("expected output to be 'false', but got %q", got)
	}

	// Set a completely new key
	_, err = executeCommand(rootCmd, "--config", configPath, "config", "set", "new_key", "new_value")
	if err != nil {
		t.Fatalf("execute set command failed: %v", err)
	}

	// Verify the new key
	output, err = executeCommand(rootCmd, "--config", configPath, "config", "get", "new_key")
	if err != nil {
		t.Fatalf("execute get command failed: %v", err)
	}
	if got := strings.TrimSpace(output); got != "new_value" {
		t.Errorf("expected output to be 'new_value', but got %q", got)
	}
}
