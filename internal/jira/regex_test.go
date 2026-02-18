package jira

import (
	"testing"
)

func TestRepoRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		matches  bool
	}{
		{
			name:     "Standard HTTPS URL",
			input:    "Repo: https://github.com/owner/repo",
			expected: "https://github.com/owner/repo",
			matches:  true,
		},
		{
			name:     "Standard HTTP URL",
			input:    "Repo: http://github.com/owner/repo",
			expected: "http://github.com/owner/repo",
			matches:  true,
		},
		{
			name:     "URL with .git suffix",
			input:    "Repo: https://github.com/owner/repo.git",
			expected: "https://github.com/owner/repo.git",
			matches:  true,
		},
		{
			name:     "Skip",
			input:    "Repo: skip",
			expected: "skip",
			matches:  true,
		},
		{
			name:     "None",
			input:    "Repo: none",
			expected: "none",
			matches:  true,
		},
		{
			name:     "Case Insensitive Skip",
			input:    "repo: SKIP",
			expected: "SKIP",
			matches:  true,
		},
		{
			name:     "Embedded in text",
			input:    "Here is the Repo: https://github.com/foo/bar for you",
			expected: "https://github.com/foo/bar",
			matches:  true,
		},
		{
			name:    "No Match",
			input:   "Repo: ",
			matches: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := RepoRegex.FindStringSubmatch(tt.input)
			if tt.matches {
				if len(matches) < 2 {
					t.Errorf("Expected match for %q, got none", tt.input)
				} else if matches[1] != tt.expected {
					t.Errorf("Expected captured group %q, got %q", tt.expected, matches[1])
				}
			} else {
				if len(matches) > 0 {
					t.Errorf("Expected no match for %q, got %v", tt.input, matches)
				}
			}
		})
	}
}
