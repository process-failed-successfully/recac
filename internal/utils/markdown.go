package utils

import (
	"strings"
)

// CleanCodeBlock strips markdown code blocks if present.
// It returns the content of the first code block found, or the original content if no block is found.
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

// CleanJSONBlock attempts to extract a JSON object or array from a string.
// It handles markdown code blocks (```json ... ```) and raw JSON wrapped in text.
func CleanJSONBlock(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// 1. Try to find ```json ... ``` (Most explicit)
	// We check this first to prioritize explicitly marked JSON blocks, even if they appear later.
	startJSON := strings.Index(input, "```json")
	if startJSON != -1 {
		contentStart := startJSON + 7 // len("```json")
		endJSON := strings.Index(input[contentStart:], "```")
		if endJSON != -1 {
			return strings.TrimSpace(input[contentStart : contentStart+endJSON])
		}
	}

	// 2. Try generic block ``` ... ```
	startBlock := strings.Index(input, "```")
	if startBlock != -1 {
		contentStart := startBlock + 3
		endBlock := strings.Index(input[contentStart:], "```")
		if endBlock != -1 {
			content := strings.TrimSpace(input[contentStart : contentStart+endBlock])

			// Remove language tag if present in the captured content
			if idx := strings.Index(content, "\n"); idx != -1 {
				firstLine := strings.TrimSpace(content[:idx])
				// If first line is short and looks like a tag, skip it
				if len(firstLine) < 10 && !strings.Contains(firstLine, " ") && !strings.Contains(firstLine, "{") && !strings.Contains(firstLine, "[") {
					return strings.TrimSpace(content[idx+1:])
				}
			}
			// If it starts with "json" and then immediate brace? (e.g. ```json{...}``` where "json" is part of content)
			if strings.HasPrefix(content, "json") {
				return strings.TrimSpace(strings.TrimPrefix(content, "json"))
			}

			return content
		}
	}

	// 3. Fallback: If it looks like a JSON object/array but has text around it
	startBrace := strings.Index(input, "{")
	startBracket := strings.Index(input, "[")

	start := -1
	if startBrace != -1 && startBracket != -1 {
		if startBrace < startBracket {
			start = startBrace
		} else {
			start = startBracket
		}
	} else if startBrace != -1 {
		start = startBrace
	} else if startBracket != -1 {
		start = startBracket
	}

	if start != -1 {
		// Find matching end
		endBrace := strings.LastIndex(input, "}")
		endBracket := strings.LastIndex(input, "]")
		end := -1

		if endBrace != -1 && endBracket != -1 {
			if endBrace > endBracket {
				end = endBrace
			} else {
				end = endBracket
			}
		} else if endBrace != -1 {
			end = endBrace
		} else if endBracket != -1 {
			end = endBracket
		}

		if end != -1 && end > start {
			return strings.TrimSpace(input[start : end+1])
		}
	}

	return input
}
