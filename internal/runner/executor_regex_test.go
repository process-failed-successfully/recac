package runner

import (
	"regexp"
	"strings"
	"testing"
)

func TestBashBlockRegex(t *testing.T) {
	regex := regexp.MustCompile("(?s)```(shell|bash|sh)?\\s+(.*?)\\s*```")

	tests := []struct {
		name     string
		input    string
		expected string
		shouldMatch bool
	}{
		{
			name:     "Standard bash",
			input:    "```bash\necho hello\n```",
			expected: "echo hello",
			shouldMatch: true,
		},
		{
			name:     "sh",
			input:    "```sh\necho hello\n```",
			expected: "echo hello",
			shouldMatch: true,
		},
		{
			name:     "shell",
			input:    "```shell\necho hello\n```",
			expected: "echo hello",
			shouldMatch: true,
		},
		{
			name:     "No language",
			input:    "```\necho hello\n```",
			expected: "echo hello",
			shouldMatch: true,
		},
		{
			name:     "No language with spaces",
			input:    "```  \necho hello\n```",
			expected: "echo hello",
			shouldMatch: true,
		},
		{
			name:     "Python block",
			input:    "```python\nprint('hello')\n```",
			shouldMatch: false,
		},
		{
			name:     "JSON block",
			input:    "```json\n{}\n```",
			shouldMatch: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matches := regex.FindAllStringSubmatch(tc.input, -1)
			if !tc.shouldMatch {
				if len(matches) > 0 {
					// Check if it matched "python" as content?
					// If input is ```python...```
					// Group 1: empty
					// \s+ : ???
					// if regex matches, it's bad.
					t.Errorf("Expected NO match for %s, but got %v", tc.name, matches)
				}
				return
			}

			if len(matches) == 0 {
				t.Fatalf("No match found for %s", tc.name)
			}
			// In my implementation, match[2] is the content
			content := strings.TrimSpace(matches[0][2])
			if content != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, content)
			}
		})
	}
}
