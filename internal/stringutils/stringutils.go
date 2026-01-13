package stringutils

import (
	"regexp"
	"strings"
)

var RepoRegex = regexp.MustCompile(`(?i)Repo: (https?://\S+)`)

func ExtractRepoURL(text string) string {
	matches := RepoRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		repoURL := strings.TrimSuffix(matches[1], ".git")
		return strings.TrimSuffix(repoURL, "/")
	}
	return ""
}
