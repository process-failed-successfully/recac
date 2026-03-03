package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestConfigSearchCmd(t *testing.T) {
	// Setup test keys
	viper.Reset()
	defer viper.Reset()

	viper.Set("agent.provider", "openai")
	viper.Set("agent.model", "gpt-4o")
	viper.Set("notifications.slack.enabled", "true")
	viper.Set("secret_api_token", "super-secret-value") // Should be redacted

	tests := []struct {
		name          string
		args          []string
		expectedOut   []string
		unexpectedOut []string
	}{
		{
			name:          "exact match",
			args:          []string{"search", "agent.provider"},
			expectedOut:   []string{"agent.provider\topenai"},
			unexpectedOut: []string{"agent.model", "notifications"},
		},
		{
			name:          "partial match",
			args:          []string{"search", "agent"},
			expectedOut:   []string{"agent.provider\topenai", "agent.model\tgpt-4o"},
			unexpectedOut: []string{"notifications.slack.enabled"},
		},
		{
			name:          "case insensitive search",
			args:          []string{"search", "SLACK"},
			expectedOut:   []string{"notifications.slack.enabled\ttrue"},
			unexpectedOut: []string{"agent.provider"},
		},
		{
			name:          "no match",
			args:          []string{"search", "nonexistent"},
			expectedOut:   []string{"No configuration keys found matching \"nonexistent\"."},
			unexpectedOut: []string{"agent.provider"},
		},
		{
			name:          "sensitive redaction",
			args:          []string{"search", "token"},
			expectedOut:   []string{"secret_api_token\t[REDACTED]"},
			unexpectedOut: []string{"super-secret-value"},
		},
		{
			name:          "sensitive redaction bypassed with flag",
			args:          []string{"search", "token", "--show-sensitive"},
			expectedOut:   []string{"secret_api_token\tsuper-secret-value"},
			unexpectedOut: []string{"[REDACTED]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)

			cmd := configSearchCmd
			cmd.SetOut(buf)

			// Reset flag state before each run to prevent test leakage
			cmd.Flags().Set("show-sensitive", "false")

			// Parse flags explicitly for the test to pick up "--show-sensitive"
			err := cmd.Flags().Parse(tt.args[1:])
			assert.NoError(t, err)

			// Execute the RunE function directly to avoid cobra parsing quirks.
			err = cmd.RunE(cmd, []string{tt.args[1]})
			assert.NoError(t, err)

			output := buf.String()
			for _, exp := range tt.expectedOut {
				// We must check if parts exist in the output because tabwriter creates variable spacing.
				// split by tab and check both parts.
				parts := strings.Split(exp, "\t")
				for _, p := range parts {
					assert.Contains(t, output, p)
				}
			}

			for _, unexp := range tt.unexpectedOut {
				assert.NotContains(t, output, unexp)
			}
		})
	}
}
