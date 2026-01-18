package utils

import "strings"

// CleanCodeBlock strips markdown code blocks if present.
// It handles generic code blocks (e.g. ```go ... ```).
func CleanCodeBlock(content string) string {
	content = strings.TrimSpace(content)

	// Try to find markdown code blocks
	start := strings.Index(content, "```")
	if start != -1 {
		// Found a code block start
		// Skip the opening ``` and potential language identifier
		codeStart := start + 3

		// Find the end of the line to skip language identifier (e.g., ```go)
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

// CleanJSONBlock strips markdown code blocks if present, specifically for JSON.
// It also has fallback logic to find the first '{' and last '}'.
func CleanJSONBlock(content string) string {
	content = strings.TrimSpace(content)

	// Try to find markdown code blocks
	start := strings.Index(content, "```")
	if start != -1 {
		// Found a code block start
		// Check if it's ```json or just ```
		codeStart := start + 3
		if strings.HasPrefix(content[codeStart:], "json") {
			codeStart += 4
		}

		// Find the end of the block
		end := strings.Index(content[codeStart:], "```")
		if end != -1 {
			// Extract the content inside the block
			return strings.TrimSpace(content[codeStart : codeStart+end])
		}
	}

	// Fallback: If no code blocks, look for the first '{' and last '}'
	firstBrace := strings.Index(content, "{")
	lastBrace := strings.LastIndex(content, "}")
	if firstBrace != -1 && lastBrace != -1 && lastBrace > firstBrace {
		return strings.TrimSpace(content[firstBrace : lastBrace+1])
	}

	return content
}
