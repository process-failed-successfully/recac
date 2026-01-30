package utils

import "strings"

// SanitizeMermaidID sanitizes a string to be a valid Mermaid node ID.
// It replaces invalid characters with underscores.
// Invalid characters include: space, hyphen, dot, slash, backslash, asterisk, colon, ampersand, parentheses, brackets, quotes, backticks.
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
