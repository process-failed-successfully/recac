package utils

import "strings"

// SanitizeMermaidID replaces all non-alphanumeric characters (except underscores)
// with underscores to ensure valid Mermaid syntax.
func SanitizeMermaidID(id string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, id)
}
