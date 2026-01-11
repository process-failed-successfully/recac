package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoctorCmd(t *testing.T) {
	// Reset viper before each test
	viper.Reset()

	// Backup original functions
	originalCheckJira := runCheckJira
	originalCheckAIProvider := runCheckAIProvider
	originalCheckDocker := runCheckDocker
	originalCheckOrchestrator := runCheckOrchestrator

	// Restore original functions after each test
	t.Cleanup(func() {
		runCheckJira = originalCheckJira
		runCheckAIProvider = originalCheckAIProvider
		runCheckDocker = originalCheckDocker
		runCheckOrchestrator = originalCheckOrchestrator
	})

	t.Run("all checks pass", func(t *testing.T) {
		viper.Reset()
		// Set up mock config
		viper.Set("agent_provider", "gemini")
		viper.Set("api_key", "test-key")
		viper.Set("jira.url", "https://test.jira.com")
		viper.Set("jira.email", "test@user.com")
		viper.Set("jira.token", "test-token")

		// Mock all checks to succeed
		runCheckJira = func() error { return nil }
		runCheckAIProvider = func() error { return nil }
		runCheckDocker = func() error { return nil }
		runCheckOrchestrator = func() error { return nil }

		output, err := executeCommand(doctorCmd)
		require.NoError(t, err, "command output: %s", output)

		assert.Contains(t, output, "Configuration... ✓")
		assert.Contains(t, output, "AI Provider Connectivity... ✓")
		assert.Contains(t, output, "Jira Connectivity... ✓")
		assert.Contains(t, output, "Docker Service... ✓")
		assert.Contains(t, output, "Orchestrator Status... ✓")
		assert.Contains(t, output, "✅ All checks passed!")
	})

	t.Run("config check fails", func(t *testing.T) {
		viper.Reset() // No config set
		// Mock other checks to pass to isolate the failure
		runCheckJira = func() error { return nil }
		runCheckAIProvider = func() error { return nil }
		runCheckDocker = func() error { return nil }
		runCheckOrchestrator = func() error { return nil }

		output, err := executeCommand(doctorCmd)
		require.Error(t, err)
		assert.Contains(t, output, "Configuration... ✖️")
		assert.Contains(t, output, "`agent_provider` is not set")
		assert.Contains(t, output, "✖️ Doctor found 1 issue(s).")
	})

	t.Run("ai provider check fails", func(t *testing.T) {
		viper.Reset()
		viper.Set("agent_provider", "gemini")
		viper.Set("api_key", "bad-key")

		// Mock AI provider to fail
		runCheckAIProvider = func() error { return fmt.Errorf("bad api key") }
		// Mock other checks to pass
		runCheckJira = func() error { return nil }
		runCheckDocker = func() error { return nil }
		runCheckOrchestrator = func() error { return nil }

		output, err := executeCommand(doctorCmd)
		require.Error(t, err)
		assert.Contains(t, output, "AI Provider Connectivity... ✖️")
		assert.Contains(t, output, "Error: bad api key")
		assert.Contains(t, output, "✖️ Doctor found 1 issue(s).")
	})

	t.Run("multiple checks fail", func(t *testing.T) {
		viper.Reset() // No config

		// Mock multiple checks to fail
		runCheckAIProvider = func() error { return fmt.Errorf("ai failure") }
		runCheckDocker = func() error { return fmt.Errorf("docker failure") }
		// Mock other checks to pass
		runCheckJira = func() error { return nil }
		runCheckOrchestrator = func() error { return nil }

		output, err := executeCommand(doctorCmd)
		require.Error(t, err)
		assert.Contains(t, output, "Configuration... ✖️")
		assert.Contains(t, output, "AI Provider Connectivity... ✖️")
		assert.Contains(t, output, "Docker Service... ✖️")
		assert.Contains(t, output, "✖️ Doctor found 3 issue(s).")
	})

	t.Run("gemini check with http mock", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("x-goog-api-key") != "good-key" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Need to create a new function for the test to override the URL
		testCheckGemini := func(apiKey string) error {
			url := server.URL // Use mock server
			req, err := http.NewRequest("GET", url, nil)
			require.NoError(t, err)
			req.Header.Set("x-goog-api-key", apiKey)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("bad status: %d", resp.StatusCode)
			}
			return nil
		}

		// Case 1: Good key
		err := testCheckGemini("good-key")
		assert.NoError(t, err)

		// Case 2: Bad key
		err = testCheckGemini("bad-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bad status: 401")
	})
}
