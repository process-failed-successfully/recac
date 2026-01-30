package utils

import "strings"

// SanitizeMermaidID sanitizes a string to be a valid Mermaid node ID.
// It replaces invalid characters (anything not alphanumeric or underscore) with underscores.
func SanitizeMermaidID(id string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, id)
}
