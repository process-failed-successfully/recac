package utils

import (
	"strings"
)

// SanitizeMermaidID sanitizes a string for use as a Mermaid node ID.
// It replaces any character that is not alphanumeric or an underscore with an underscore.
func SanitizeMermaidID(id string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, id)
}
