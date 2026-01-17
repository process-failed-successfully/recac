package utils

import "strings"

// CleanMarkdown strips markdown code blocks (```) from a string if present.
// It returns the content inside the first code block, or the original string if no block is found.
func CleanMarkdown(content string) string {
	content = strings.TrimSpace(content)

	// Try to find markdown code blocks
	start := strings.Index(content, "```")
	if start != -1 {
		// Found a code block start
		// Skip the opening ```
		codeStart := start + 3

		// Find the end of the line to skip language identifier (e.g., ```go, ```json)
		if idx := strings.Index(content[codeStart:], "\n"); idx != -1 {
			codeStart += idx + 1
		}

		// Find the end of the block
		end := strings.Index(content[codeStart:], "```")
		if end != -1 {
			// Extract the content inside the block
			return strings.TrimSpace(content[codeStart : codeStart+end])
		}
	}

	return content
}
