package utils

import (
	"regexp"
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

// MarkdownBlock represents a parsed block of content from a markdown file.
type MarkdownBlock struct {
	Type    string // "text" or "code"
	Content string
	Lang    string // Language for code blocks, empty for text
}

// ParseMarkdownBlocks parses lines of markdown and separates them into text and code blocks.
func ParseMarkdownBlocks(lines []string) []MarkdownBlock {
	var blocks []MarkdownBlock
	var currentBuilder strings.Builder
	inCodeBlock := false
	codeLang := ""

	flushBuffer := func(t, l string) {
		if currentBuilder.Len() > 0 {
			blocks = append(blocks, MarkdownBlock{
				Type:    t,
				Content: currentBuilder.String(),
				Lang:    l,
			})
			currentBuilder.Reset()
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inCodeBlock {
				// End of code block
				// Do not include the closing fence in the content
				flushBuffer("code", codeLang)
				inCodeBlock = false
				codeLang = ""
			} else {
				// Start of code block
				// Flush previous text
				flushBuffer("text", "")
				inCodeBlock = true
				codeLang = strings.TrimPrefix(strings.TrimSpace(line), "```")
			}
		} else {
			currentBuilder.WriteString(line)
			currentBuilder.WriteString("\n")
		}
	}

	// Flush remaining
	if currentBuilder.Len() > 0 {
		if inCodeBlock {
			// Unclosed code block, treat as code
			flushBuffer("code", codeLang)
		} else {
			flushBuffer("text", "")
		}
	}

	return blocks
}
