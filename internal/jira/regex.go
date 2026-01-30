package jira

import "regexp"

// RepoRegex is a compiled regular expression for extracting repository URLs from Jira ticket descriptions.
// It matches strings like "Repo: https://github.com/owner/repo" or "Repo: `https://github.com/owner/repo`".
// We use \x60 for backtick to avoid string literal issues.
var RepoRegex = regexp.MustCompile(`(?i)Repo: \x60?(https?://[^\x60\s]+)\x60?`)
