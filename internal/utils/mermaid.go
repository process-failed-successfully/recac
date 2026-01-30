package utils

import (
	"strings"
	"unicode"
)

// SanitizeMermaidID sanitizes a string to be a valid Mermaid node ID.
// It replaces invalid characters with underscores.
// Valid characters are alphanumeric and underscores.
func SanitizeMermaidID(id string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return r
		}
		return '_'
	}, id)
}
