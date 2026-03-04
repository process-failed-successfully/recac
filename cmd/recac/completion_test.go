package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionCmd(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedStr   string
		expectedError bool
	}{
		{
			name:          "generate bash completion",
			args:          []string{"bash"},
			expectedStr:   "# bash completion for recac",
			expectedError: false,
		},
		{
			name:          "generate zsh completion",
			args:          []string{"zsh"},
			expectedStr:   "#compdef recac",
			expectedError: false,
		},
		{
			name:          "generate fish completion",
			args:          []string{"fish"},
			expectedStr:   "complete -c recac",
			expectedError: false,
		},
		{
			name:          "generate powershell completion",
			args:          []string{"powershell"},
			expectedStr:   "Register-ArgumentCompleter",
			expectedError: false,
		},
		{
			name:          "invalid shell argument",
			args:          []string{"invalid-shell"},
			expectedStr:   "",
			expectedError: true,
		},
		{
			name:          "no arguments",
			args:          []string{},
			expectedStr:   "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Temporarily replace os.Stdout
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)
			os.Stdout = w

			// Ensure stdout is restored even on panic
			defer func() {
				w.Close()
				os.Stdout = oldStdout
			}()

			output, err := executeCommand(rootCmd, append([]string{"completion"}, tt.args...)...)

			w.Close()
			os.Stdout = oldStdout

			var stdoutBuf bytes.Buffer
			_, _ = stdoutBuf.ReadFrom(r)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// The executeCommand output might be empty because we bypassed SetOut by using os.Stdout directly in the command.
				// However, the pipe will catch it.
				combinedOutput := output + stdoutBuf.String()
				assert.Contains(t, combinedOutput, tt.expectedStr)
			}
		})
	}
}
