package utils

import "strings"

// SanitizeMermaidID sanitizes a string to be used as a Mermaid node ID.
// It replaces characters invalid in Mermaid IDs with underscores.
func SanitizeMermaidID(id string) string {
	replacer := strings.NewReplacer(
		"-", "_",
		" ", "_",
		".", "_",
		"/", "_",
		"\\", "_",
		"*", "_",
		":", "_",
		"&", "_",
		"(", "_",
		")", "_",
		"[", "_",
		"]", "_",
		"\"", "_",
		"'", "_",
		"`", "_",
	)
	return replacer.Replace(id)
}
