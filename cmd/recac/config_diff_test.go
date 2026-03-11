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
	viper.Set("provider", "openai") // Default is gemini
	viper.Set("max_iterations", 50) // Default is 20
	viper.Set("custom.setting", "my-value") // Not in defaults
	viper.Set("secret_token", "super-secret") // Not in defaults, but sensitive
	viper.Set("jira.url", "https://custom.jira.com") // Sensitive check is just url here?
	viper.Set("model", "gemini-pro") // Unchanged, shouldn't appear
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
				"super-secret", // Should be redacted
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
				"[REDACTED]", // Flag should reveal
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

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		val      interface{}
		expected bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"non-empty string", "test", false},
		{"empty array", []int{}, true},
		{"non-empty array", []int{1}, false},
		{"empty map", map[string]int{}, true},
		{"non-empty map", map[string]int{"a": 1}, false},
		{"false bool", false, true},
		{"true bool", true, false},
		{"zero int", 0, true},
		{"non-zero int", 1, false},
		{"zero int8", int8(0), true},
		{"non-zero int8", int8(1), false},
		{"zero int16", int16(0), true},
		{"non-zero int16", int16(1), false},
		{"zero int32", int32(0), true},
		{"non-zero int32", int32(1), false},
		{"zero int64", int64(0), true},
		{"non-zero int64", int64(1), false},
		{"zero uint", uint(0), true},
		{"non-zero uint", uint(1), false},
		{"zero uint8", uint8(0), true},
		{"non-zero uint8", uint8(1), false},
		{"zero uint16", uint16(0), true},
		{"non-zero uint16", uint16(1), false},
		{"zero uint32", uint32(0), true},
		{"non-zero uint32", uint32(1), false},
		{"zero uint64", uint64(0), true},
		{"non-zero uint64", uint64(1), false},
		{"zero uintptr", uintptr(0), true},
		{"non-zero uintptr", uintptr(1), false},
		{"zero float32", float32(0), true},
		{"non-zero float32", float32(1.1), false},
		{"zero float64", float64(0), true},
		{"non-zero float64", float64(1.1), false},
		{"nil ptr", (*int)(nil), true},
		{"non-nil ptr", new(int), false},
		{"struct", struct{}{}, false}, // Default false for unhandled types
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isEmpty(tt.val))
		})
	}
}
