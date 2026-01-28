package utils

import (
	"regexp"
	"strings"
)

// CleanCodeBlock strips markdown code blocks if present.
// It returns the content of the first code block found, or the original content if no block is found.
func CleanCodeBlock(content string) string {
	content = strings.TrimSpace(content)

	// Try regex for ``` ... ```
	match := reBlock.FindStringSubmatch(content)
	if len(match) > 1 {
		inner := strings.TrimSpace(match[1])
		// Remove potential language tag
		if idx := strings.Index(inner, "\n"); idx != -1 {
			firstLine := strings.TrimSpace(inner[:idx])
			// Heuristic: if first line is short and no spaces, assume it's language tag
			if len(firstLine) < 20 && !strings.Contains(firstLine, " ") {
				return strings.TrimSpace(inner[idx+1:])
			}
		}
		return inner
	}

	return content
}

var (
	reJSONBlock = regexp.MustCompile("(?s)```json(.*?)```")
	reBlock     = regexp.MustCompile("(?s)```(.*?)```")
)

// CleanJSONBlock attempts to extract a JSON object or array from a string.
// It handles markdown code blocks (```json ... ```) and raw JSON wrapped in text.
func CleanJSONBlock(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// 1. Try regex for ```json ... ``` (Most explicit)
	match := reJSONBlock.FindStringSubmatch(input)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	// 2. Try regex for ``` ... ``` (Any block)
	match2 := reBlock.FindStringSubmatch(input)
	if len(match2) > 1 {
		content := strings.TrimSpace(match2[1])
		// Remove language tag if present in the captured content
		if idx := strings.Index(content, "\n"); idx != -1 {
			firstLine := strings.TrimSpace(content[:idx])
			// If first line is short and looks like a tag, skip it
			if len(firstLine) < 10 && !strings.Contains(firstLine, " ") && !strings.Contains(firstLine, "{") && !strings.Contains(firstLine, "[") {
				return strings.TrimSpace(content[idx+1:])
			}
		}
		// If it starts with "json" and then immediate brace?
		if strings.HasPrefix(content, "json") {
			return strings.TrimSpace(strings.TrimPrefix(content, "json"))
		}

		return content
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
