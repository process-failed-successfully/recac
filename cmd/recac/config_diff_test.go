package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestConfigDiffCmd(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	// Set overrides
	viper.Set("provider", "openai")                  // Default is gemini
	viper.Set("max_iterations", 50)                  // Default is 20
	viper.Set("custom.setting", "my-value")          // Not in defaults
	viper.Set("secret_token", "super-secret")        // Not in defaults, but sensitive
	viper.Set("jira.url", "https://custom.jira.com") // Sensitive check is just url here?
	viper.Set("model", "gemini-pro")                 // Unchanged, shouldn't appear
	// Wait, is 'model' gemini-pro? The defaults map has "model": "gemini-pro". So it shouldn't appear.

	tests := []struct {
		name          string
		args          []string
		expectedOut   []string
		unexpectedOut []string
	}{
		{
			name: "shows overrides",
			args: []string{"diff"},
			expectedOut: []string{
				"provider\topenai\tgemini",
				"max_iterations\t50\t20",
				"custom.setting\tmy-value\t<none>",
				"secret_token\t[REDACTED]\t<none>",
			},
			unexpectedOut: []string{
				"model\tgemini-pro\tgemini-pro", // unchanged, shouldn't show
				"super-secret",                  // Should be redacted
			},
		},
		{
			name: "shows overrides with sensitive data revealed",
			args: []string{"diff", "--show-sensitive"},
			expectedOut: []string{
				"provider\topenai\tgemini",
				"max_iterations\t50\t20",
				"custom.setting\tmy-value\t<none>",
				"secret_token\tsuper-secret\t<none>",
			},
			unexpectedOut: []string{
				"model\tgemini-pro\tgemini-pro", // unchanged, shouldn't show
				"[REDACTED]",                    // Flag should reveal
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			cmd := configDiffCmd
			cmd.SetOut(buf)

			cmd.Flags().Set("show-sensitive", "false")
			err := cmd.Flags().Parse(tt.args[1:])
			assert.NoError(t, err)

			err = cmd.RunE(cmd, []string{})
			assert.NoError(t, err)

			output := buf.String()
			for _, exp := range tt.expectedOut {
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
