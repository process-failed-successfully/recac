package agent

import (
	"os"
	"testing"
)

func TestOpenRouterClient_MaxTokens_CI(t *testing.T) {
	// Save original env
	origCI := os.Getenv("CI")
	defer os.Setenv("CI", origCI)

	// Case 1: CI is true
	os.Setenv("CI", "true")
	client := NewOpenRouterClient("key", "model", "project")
	// Access the embedded BaseClient's DefaultMaxTokens
	if client.BaseClient.DefaultMaxTokens != 1000 {
		t.Errorf("Expected DefaultMaxTokens to be 1000 in CI, got %d", client.BaseClient.DefaultMaxTokens)
	}

	// Case 2: CI is false/unset
	os.Setenv("CI", "false")
	client = NewOpenRouterClient("key", "model", "project")
	if client.BaseClient.DefaultMaxTokens != 128000 {
		t.Errorf("Expected DefaultMaxTokens to be 128000 when CI is not true, got %d", client.BaseClient.DefaultMaxTokens)
	}
}
