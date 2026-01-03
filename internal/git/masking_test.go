package git

import (
	"bytes"
	"testing"
)

func TestMaskingWriter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "GitHub PAT",
			input:    "Cloning into https://ghp_1234567890abcdef@github.com/org/repo.git",
			expected: "Cloning into https://[REDACTED]@github.com/org/repo.git",
		},
		{
			name:     "Basic Auth",
			input:    "remote: https://user:pass@example.com/repo.git",
			expected: "remote: https://[REDACTED]@example.com/repo.git",
		},
		{
			name:     "Normal URL",
			input:    "Cloning into https://github.com/org/repo.git",
			expected: "Cloning into https://github.com/org/repo.git",
		},
		{
			name:     "Standard Output",
			input:    "Already up to date.",
			expected: "Already up to date.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			mw := &maskingWriter{w: &buf}
			mw.Write([]byte(tt.input))
			if buf.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, buf.String())
			}
		})
	}
}
