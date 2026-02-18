package jira

import "regexp"

// RepoRegex is a compiled regular expression for extracting repository URLs from Jira ticket descriptions.
// It matches strings like "Repo: https://github.com/owner/repo" or "Repo: skip".
// Updated to be more flexible, matching any non-whitespace string after "Repo: "
var RepoRegex = regexp.MustCompile(`(?i)Repo: (\S+)`)
