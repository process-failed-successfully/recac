package utils

import "strings"

// SanitizeMermaidID replaces invalid characters in a Mermaid ID with underscores.
// Invalid characters include: space, hyphen, dot, slash, backslash, asterisk,
// colon, ampersand, parentheses, brackets, quotes, and backticks.
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
