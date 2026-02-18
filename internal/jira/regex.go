package jira

import "regexp"

// RepoRegex is a compiled regular expression for extracting repository URLs from Jira ticket descriptions.
// It matches strings like "Repo: https://github.com/owner/repo", "Repo: skip", or "Repo: none".
var RepoRegex = regexp.MustCompile(`(?i)Repo: ((?:https?://\S+)|skip|none)`)
