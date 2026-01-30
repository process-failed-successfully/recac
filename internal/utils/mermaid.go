package utils

import "strings"

// SanitizeMermaidID cleans an ID string for use in Mermaid diagrams.
// It replaces spaces, dashes, and dots with underscores.
func SanitizeMermaidID(id string) string {
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, ".", "_")
	return id
}
