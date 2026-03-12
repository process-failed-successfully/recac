package main

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestAliasExpansion(t *testing.T) {
	// Setup mock aliases in viper
	viper.Set("orchestrator.aliases", map[string]string{
		"ls": "--list-jobs",
		"lp": "--list-pending",
		"st": "--status",
		"cj": "--cancel-job=123", // multi-word alias replacement
	})
	defer viper.Reset()

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "Single word alias expansion",
			input:    []string{"orchestrator", "ls"},
			expected: []string{"orchestrator", "--list-jobs"},
		},
		{
			name:     "No expansion when no match",
			input:    []string{"orchestrator", "--list-jobs"},
			expected: []string{"orchestrator", "--list-jobs"},
		},
		{
			name:     "Multiple alias expansion (only first should expand)",
			input:    []string{"orchestrator", "ls", "st"},
			expected: []string{"orchestrator", "--list-jobs", "st"}, // st is not expanded because it's not the target
		},
		{
			name:     "Alias with spaces",
			input:    []string{"orchestrator", "cj"},
			expected: []string{"orchestrator", "--cancel-job=123"},
		},
		{
			name:     "Config flag skipped",
			input:    []string{"orchestrator", "--config", "config.yaml", "ls"},
			expected: []string{"orchestrator", "--config", "config.yaml", "--list-jobs"},
		},
		{
			name:     "Config flag skipped (short)",
			input:    []string{"orchestrator", "-c", "config.yaml", "ls"},
			expected: []string{"orchestrator", "-c", "config.yaml", "--list-jobs"},
		},
		{
			name:     "Config flag skipped (equals)",
			input:    []string{"orchestrator", "--config=config.yaml", "ls"},
			expected: []string{"orchestrator", "--config=config.yaml", "--list-jobs"},
		},
		{
			name:     "Alias follows flags",
			input:    []string{"orchestrator", "--verbose", "ls"},
			expected: []string{"orchestrator", "--verbose", "--list-jobs"},
		},
		{
			name:     "Alias follows flag with value",
			input:    []string{"orchestrator", "--config", "my.yaml", "ls"},
			expected: []string{"orchestrator", "--config", "my.yaml", "--list-jobs"},
		},
		{
			name:     "Target index doesn't exist",
			input:    []string{"orchestrator"},
			expected: []string{"orchestrator"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandAliases(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
