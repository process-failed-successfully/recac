package utils

import "strings"

// SanitizeMermaidID sanitizes a string for use as a Mermaid node ID.
// It replaces invalid characters with underscores.
func SanitizeMermaidID(id string) string {
	replacer := strings.NewReplacer(
		" ", "_",
		"-", "_",
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
