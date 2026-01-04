package main

import (
	"regexp"
	"testing"
)

func TestRepoRegex(t *testing.T) {
	repoRegex := regexp.MustCompile(`(?i)Repo: (https?://\S+)`)

	tests := []struct {
		description string
		wantMatch   bool
		wantURL     string
	}{
		{
			description: "Some description. Repo: https://github.com/user/repo",
			wantMatch:   true,
			wantURL:     "https://github.com/user/repo",
		},
		{
			description: "Some description. repo: http://gitlab.com/user/repo.git",
			wantMatch:   true,
			wantURL:     "http://gitlab.com/user/repo.git",
		},
		{
			description: "No repo here",
			wantMatch:   false,
		},
		{
			description: "Repo:not-a-url",
			wantMatch:   false,
		},
	}

	for _, tt := range tests {
		matches := repoRegex.FindStringSubmatch(tt.description)
		gotMatch := len(matches) > 1
		if gotMatch != tt.wantMatch {
			t.Errorf("MatchString(%q) = %v, want %v", tt.description, gotMatch, tt.wantMatch)
		}
		if tt.wantMatch && matches[1] != tt.wantURL {
			t.Errorf("Extract URL from %q: got %q, want %q", tt.description, matches[1], tt.wantURL)
		}
	}
}
