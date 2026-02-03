package utils

import (
	"regexp"
	"strings"
)

var (
	reCodeBlock = regexp.MustCompile("(?s)```(?:.*?\n)?(.*?)```")
	reJSONBlock = regexp.MustCompile("(?s)```json(.*?)```")
)

// CleanCodeBlock strips markdown code blocks if present.
// It returns the content of the first code block found, or the original content if no block is found.
func CleanCodeBlock(content string) string {
	content = strings.TrimSpace(content)

	match := reCodeBlock.FindStringSubmatch(content)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
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

	// 1. Try explicit regex for ```json ... ``` (Priority for explicit json)
	match := reJSONBlock.FindStringSubmatch(input)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	// 2. Fallback: Use generic CleanCodeBlock to strip any other wrappers
	// Check if it was a block to safely strip "json" prefix if it was inside
	isBlock := reCodeBlock.MatchString(input)
	input = CleanCodeBlock(input)

	if isBlock && strings.HasPrefix(input, "json") {
		input = strings.TrimSpace(strings.TrimPrefix(input, "json"))
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
