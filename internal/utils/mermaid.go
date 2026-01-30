package utils

import "strings"

// SanitizeMermaidID sanitizes a string to be used as a Mermaid node ID.
// It replaces any non-alphanumeric character (except underscore) with an underscore.
func SanitizeMermaidID(id string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, id)
}
