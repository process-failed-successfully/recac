package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigPathCommand(t *testing.T) {
	tests := []struct {
		name         string
		mockPath     string
		expectedPath string
	}{
		{
			name:         "Returns valid path",
			mockPath:     "/home/user/.recac.yaml",
			expectedPath: "/home/user/.recac.yaml\n",
		},
		{
			name:         "Returns default path when empty",
			mockPath:     "",
			expectedPath: "config.yaml\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalViperConfigFileUsed := viperConfigFileUsed
			viperConfigFileUsed = func() string {
				return tt.mockPath
			}
			defer func() { viperConfigFileUsed = originalViperConfigFileUsed }()

			output, err := executeCommand(rootCmd, "config", "path")
			require.NoError(t, err)
			assert.Equal(t, tt.expectedPath, output)
		})
	}
}
