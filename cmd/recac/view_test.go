package main

import (
	"bytes"
	"github.com/spf13/viper"
	"os"
	"strings"
	"testing"
)

func Test_printConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        map[string]interface{}
		showSensitive bool
		expected      string
	}{
		{
			name: "sensitive values redacted",
			config: map[string]interface{}{
				"gemini_api_key": "test-key",
				"jira_api_token": "test-token",
				"some_secret":    "test-secret",
				"log_level":      "info",
			},
			showSensitive: false,
			expected:      "gemini_api_key: [REDACTED]\njira_api_token: [REDACTED]\nlog_level: info\nsome_secret: [REDACTED]\n",
		},
		{
			name: "sensitive values shown",
			config: map[string]interface{}{
				"gemini_api_key": "test-key",
				"jira_api_token": "test-token",
				"some_secret":    "test-secret",
				"log_level":      "info",
			},
			showSensitive: true,
			expected:      "gemini_api_key: test-key\njira_api_token: test-token\nlog_level: info\nsome_secret: test-secret\n",
		},
		{
			name: "nested sensitive values",
			config: map[string]interface{}{
				"credentials": map[string]interface{}{
					"github_token": "nested-token",
				},
				"log_level": "debug",
			},
			showSensitive: false,
			expected:      "credentials.github_token: [REDACTED]\nlog_level: debug\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printConfig(&buf, tt.config, "", tt.showSensitive)
			if buf.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, buf.String())
			}
		})
	}
}

func TestConfigViewCommand(t *testing.T) {
	// Reset viper to avoid test pollution
	viper.Reset()

	// Set up a temporary config file
	const testConfig = `
gemini_api_key: "test-key"
jira_api_token: "test-token"
log_level: "info"
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(testConfig); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test case 1: Redacted output
	t.Run("redacted output", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "--config", tmpfile.Name(), "config", "view")
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}
		if !strings.Contains(output, "gemini_api_key: [REDACTED]") {
			t.Errorf("expected output to contain 'gemini_api_key: [REDACTED]', but it did not. Full output:\n%s", output)
		}
		if !strings.Contains(output, "jira_api_token: [REDACTED]") {
			t.Errorf("expected output to contain 'jira_api_token: [REDACTED]', but it did not. Full output:\n%s", output)
		}
		if !strings.Contains(output, "log_level: info") {
			t.Errorf("expected output to contain 'log_level: info', but it did not. Full output:\n%s", output)
		}
	})

	// Test case 2: Show sensitive output
	t.Run("show sensitive output", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "--config", tmpfile.Name(), "config", "view", "--show-sensitive")
		if err != nil {
			t.Fatalf("command failed: %v", err)
		}
		if !strings.Contains(output, "gemini_api_key: test-key") {
			t.Errorf("expected output to contain 'gemini_api_key: test-key', but it did not. Full output:\n%s", output)
		}
		if !strings.Contains(output, "jira_api_token: test-token") {
			t.Errorf("expected output to contain 'jira_api_token: test-token', but it did not. Full output:\n%s", output)
		}
	})
}
