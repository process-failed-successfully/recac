package orchestrator

import (
	"regexp"
	"strings"
)

// extractRepoURL extracts a repository URL from text using the provided regex.
// It removes the ".git" suffix if present.
func extractRepoURL(text string, repoRegex *regexp.Regexp) string {
	if repoRegex == nil {
		return ""
	}
	matches := repoRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSuffix(matches[1], ".git")
	}
	return ""
}

// sanitizeK8sName sanitizes a string to be used as a Kubernetes resource name.
// It converts to lowercase and replaces non-alphanumeric characters with hyphens.
func sanitizeK8sName(name string) string {
	// Lowercase and replace non-alphanumeric with -
	name = strings.ToLower(name)
	k8sNameSanitizerRegex := regexp.MustCompile("[^a-z0-9]+")
	name = k8sNameSanitizerRegex.ReplaceAllString(name, "-")
	return strings.Trim(name, "-")
}

// boolPtr returns a pointer to the given boolean value.
func boolPtr(b bool) *bool {
	return &b
}
