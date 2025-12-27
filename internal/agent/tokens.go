package agent

import (
	"strconv"
	"strings"
)

// EstimateTokenCount estimates the number of tokens in a text string.
// Uses approximate counting: ~4 characters per token for English text.
// This is a rough approximation; actual tokenization varies by model.
func EstimateTokenCount(text string) int {
	if text == "" {
		return 0
	}
	// Remove extra whitespace and count
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	// Approximate: 4 characters per token for English
	// Add 1 for every 4 characters, plus some overhead for punctuation/spaces
	charCount := len([]rune(trimmed))
	return (charCount / 4) + 1
}

// TruncateToTokenLimit truncates text to fit within a token limit while preserving important context.
// It attempts to keep the beginning and end of the text, removing from the middle.
func TruncateToTokenLimit(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}

	currentTokens := EstimateTokenCount(text)
	if currentTokens <= maxTokens {
		return text
	}

	// Reserve tokens for truncation marker (approximately 5 tokens)
	truncationMarker := "\n[... truncated ...]\n"
	markerTokens := EstimateTokenCount(truncationMarker)
	availableTokens := maxTokens - markerTokens
	if availableTokens <= 0 {
		// If marker itself exceeds limit, return empty
		return ""
	}

	// Calculate max characters we can keep (reserve 50% for start, 50% for end)
	maxChars := availableTokens * 4 // 4 chars per token
	maxStartChars := maxChars / 2
	maxEndChars := maxChars / 2

	// Split into lines to preserve structure
	lines := strings.Split(text, "\n")
	if len(lines) <= 1 {
		// Single line: truncate from middle
		runes := []rune(text)
		if len(runes) <= maxChars {
			// If entire text fits, return as-is
			return text
		}
		// Keep start and end portions
		startPortion := string(runes[:min(maxStartChars, len(runes))])
		endPortion := ""
		if len(runes) > maxStartChars {
			endStart := max(len(runes)-maxEndChars, maxStartChars)
			endPortion = string(runes[endStart:])
		}
		result := startPortion + truncationMarker + endPortion
		
		// Verify and recursively truncate if needed
		if EstimateTokenCount(result) > maxTokens {
			return TruncateToTokenLimit(result, maxTokens)
		}
		return result
	}

	// Multi-line: remove lines from middle
	// Calculate how many characters we can keep from start and end lines
	startChars := 0
	startLines := []string{}
	for i := 0; i < len(lines) && startChars < maxStartChars; i++ {
		lineChars := len([]rune(lines[i]))
		if startChars+lineChars+1 > maxStartChars { // +1 for newline
			break
		}
		startLines = append(startLines, lines[i])
		startChars += lineChars + 1
	}

	endChars := 0
	endLines := []string{}
	for i := len(lines) - 1; i >= 0 && endChars < maxEndChars; i-- {
		// Don't overlap with start lines
		if len(startLines) > 0 && i < len(startLines) {
			break
		}
		lineChars := len([]rune(lines[i]))
		if endChars+lineChars+1 > maxEndChars { // +1 for newline
			break
		}
		endLines = append([]string{lines[i]}, endLines...)
		endChars += lineChars + 1
	}

	omittedCount := len(lines) - len(startLines) - len(endLines)
	result := strings.Join(startLines, "\n") + "\n[... truncated " + strconv.Itoa(omittedCount) + " lines ...]\n" + strings.Join(endLines, "\n")

	// Verify we're under the limit (recursive safety)
	if EstimateTokenCount(result) > maxTokens {
		// If still too large, be more aggressive
		return TruncateToTokenLimit(result, maxTokens*90/100)
	}

	return result
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SummarizeForTokenLimit creates a summary when text exceeds the token limit significantly.
// This is a simple implementation that extracts key information.
func SummarizeForTokenLimit(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}

	if EstimateTokenCount(text) <= maxTokens {
		return text
	}

	// Simple summarization: extract first paragraph, key sentences, and last paragraph
	paragraphs := strings.Split(text, "\n\n")
	if len(paragraphs) == 0 {
		return TruncateToTokenLimit(text, maxTokens)
	}

	// Reserve tokens for summary marker
	omittedMarker := "[... %d paragraphs omitted ...]"
	markerTokens := EstimateTokenCount(omittedMarker) + 5 // Reserve extra for number
	availableTokens := maxTokens - markerTokens
	if availableTokens <= 0 {
		// If marker itself exceeds limit, just truncate
		return TruncateToTokenLimit(text, maxTokens)
	}

	var summary strings.Builder
	
	// Calculate how many tokens we can use for first and last paragraphs
	tokensPerParagraph := availableTokens / 2
	
	// Add first paragraph (truncated if needed)
	if len(paragraphs) > 0 {
		firstPara := paragraphs[0]
		if EstimateTokenCount(firstPara) > tokensPerParagraph {
			firstPara = TruncateToTokenLimit(firstPara, tokensPerParagraph)
		}
		if firstPara != "" {
			summary.WriteString(firstPara)
			summary.WriteString("\n\n")
		}
	}

	// Add middle summary if there are multiple paragraphs
	if len(paragraphs) > 2 {
		omittedCount := len(paragraphs) - 2
		summary.WriteString("[... " + strconv.Itoa(omittedCount) + " paragraphs omitted ...]\n\n")
	}

	// Add last paragraph (truncated if needed)
	if len(paragraphs) > 1 {
		lastPara := paragraphs[len(paragraphs)-1]
		if EstimateTokenCount(lastPara) > tokensPerParagraph {
			lastPara = TruncateToTokenLimit(lastPara, tokensPerParagraph)
		}
		if lastPara != "" {
			summary.WriteString(lastPara)
		}
	}

	result := summary.String()
	
	// If result is empty, fall back to truncation
	if result == "" {
		return TruncateToTokenLimit(text, maxTokens)
	}
	
	// Ensure we're still under limit
	if EstimateTokenCount(result) > maxTokens {
		return TruncateToTokenLimit(result, maxTokens)
	}

	return result
}
