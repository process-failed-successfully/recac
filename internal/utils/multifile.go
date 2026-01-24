package utils

import (
	"regexp"
	"strings"
)

var (
	// Regex to match <file path="..."> content </file>
	// (?s) enables dot to match newlines
	reFileBlock = regexp.MustCompile(`(?s)<file\s+path="([^"]+)">\s*(.*?)\s*</file>`)
)

// ParseFileBlocks extracts content wrapped in <file path="...">...</file> tags.
// Returns a map of file path to content.
// It trims whitespace from the extracted content.
func ParseFileBlocks(input string) map[string]string {
	result := make(map[string]string)

	matches := reFileBlock.FindAllStringSubmatch(input, -1)

	for _, match := range matches {
		if len(match) == 3 {
			path := match[1]
			content := strings.TrimSpace(match[2])
			// Ensure it ends with a newline if it's not empty, as most editors/linters expect
			if content != "" && !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			result[path] = content
		}
	}

	return result
}
