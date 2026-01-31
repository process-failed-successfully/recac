package utils

import (
	"regexp"
	"strings"
)

// RepoRegex matches strings like "Repo: https://github.com/owner/repo".
var RepoRegex = regexp.MustCompile(`(?i)Repo: (https?://\S+)`)

// ExtractRepoURL extracts the repository URL from text using the provided regex.
func ExtractRepoURL(text string, repoRegex *regexp.Regexp) string {
	if repoRegex == nil {
		return ""
	}
	matches := repoRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSuffix(matches[1], ".git")
	}
	return ""
}
