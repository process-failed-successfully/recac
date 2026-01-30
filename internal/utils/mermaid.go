package utils

import "strings"

// SanitizeMermaidID replaces characters invalid in Mermaid IDs with underscores.
// It handles space, hyphen, dot, slash, backslash, asterisk, colon, ampersand,
// parentheses, brackets, and quotes.
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
		"`", "_", // Also sanitize backticks as they can be problematic
	)
	return replacer.Replace(id)
}
