package utils

import (
	"regexp"
)

var mermaidIDRegex = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// SanitizeMermaidID sanitizes a string for use as a Mermaid node ID.
// It replaces any character that is not alphanumeric or an underscore with an underscore.
func SanitizeMermaidID(id string) string {
	return mermaidIDRegex.ReplaceAllString(id, "_")
}
