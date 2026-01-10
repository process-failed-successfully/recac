package agent

import (
	"strconv"
	"strings"
)

// EstimateTokenCount estimates the number of tokens in a text string.
// Uses approximate counting: ~4 characters per token for English text.
// This is a rough approximation; actual tokenization varies by model.
func EstimateTokenCount(text string) int {
	n := len(text)
	if n == 0 {
		return 0
	}
	// Approximate: 4 characters per token for English.
	// We use len(text) (byte count) instead of RuneCountInString for performance (O(1) vs O(N)).
	// For ASCII, bytes == runes. For UTF-8, bytes >= runes, so this slightly overestimates, which is safer for limits.
	return (n / 4) + 1
}

// TruncateToTokenLimit truncates text to fit within a token limit while preserving important context.
// It attempts to keep the beginning and end of the text, removing from the middle.
func TruncateToTokenLimit(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}

	if EstimateTokenCount(text) <= maxTokens {
		return text
	}

	// Reserve tokens for truncation marker (approximately 10 tokens for the verbose one)
	truncationMarker := "\n[... truncated ...]\n"
	markerTokens := EstimateTokenCount(truncationMarker)
	availableTokens := maxTokens - markerTokens
	if availableTokens <= 0 {
		// If marker itself exceeds limit, perform a hard truncate.
		if maxTokens <= 1 {
			// Not enough space for even one token if we truncate, so return empty.
			return ""
		}
		// (maxTokens-1)*4 ensures that (n/4)+1 <= maxTokens
		hardMaxChars := (maxTokens - 1) * 4
		if len(text) > hardMaxChars {
			return text[:hardMaxChars]
		}
		return text
	}

	// Calculate max characters we can keep (reserve 50% for start, 50% for end)
	maxChars := availableTokens * 4 // 4 chars per token
	maxStartChars := maxChars / 2
	maxEndChars := maxChars / 2

	n := len(text)

	// Single line check or no newlines
	firstNewLine := strings.IndexByte(text, '\n')
	if firstNewLine == -1 {
		// Single line: truncate from middle.
		// We use rune slicing here to ensure we don't split multi-byte characters.
		runes := []rune(text)
		if len(runes) <= maxChars {
			return text
		}
		startPortion := string(runes[:min(maxStartChars, len(runes))])
		endPortion := ""
		if len(runes) > maxStartChars {
			endStart := max(len(runes)-maxEndChars, maxStartChars)
			endPortion = string(runes[endStart:])
		}
		result := startPortion + truncationMarker + endPortion
		if EstimateTokenCount(result) > maxTokens {
			return TruncateToTokenLimit(result, maxTokens)
		}
		return result
	}

	// Multi-line logic:

	// 1. Find start cut point
	startCut := 0
	if maxStartChars >= n {
		startCut = n
	} else {
		// Scan forward finding newlines until we exceed maxStartChars.
		scanPos := 0
		for {
			nextNL := strings.IndexByte(text[scanPos:], '\n')
			if nextNL == -1 {
				// No more newlines. Check if the rest fits
				if len(text) <= maxStartChars {
					startCut = len(text)
				}
				break
			}
			absoluteNL := scanPos + nextNL

			// Length if we include this line (up to and including newline)
			if absoluteNL+1 > maxStartChars {
				break
			}

			// This line fits.
			startCut = absoluteNL + 1 // Include the newline
			scanPos = absoluteNL + 1
		}
	}

	// 2. Find end cut point
	endCut := n
	if maxEndChars >= n {
		endCut = 0
	} else {
		// Loop backwards finding lines that fit in maxEndChars
		currLen := 0
		currentEndCut := n

		cursor := n
		for {
			p := -1
			if cursor > 0 {
				p = strings.LastIndexByte(text[:cursor], '\n')
			}

			// Line is from p+1 to cursor.

			realLen := cursor - (p + 1)

			// If we keep this line, we keep content + newline.
			// Logic matches `lineChars + 1`.

			if currLen+realLen+1 > maxEndChars {
				break
			}

			currLen += realLen + 1
			currentEndCut = p + 1

			if p == -1 {
				break // Reached start of string
			}
			cursor = p // Move cursor to the newline we just found
		}
		endCut = currentEndCut
	}

	// Check overlap
	if startCut >= endCut {
		// Fallback for when the text is too short for the start/end logic to work without overlap.
		// Simply truncate from the end.
		safeCut := maxTokens * 4
		if safeCut > len(text) {
			safeCut = len(text)
		}
		return text[:safeCut]
	}

	// Omitted lines count
	omittedCount := 0
	dropped := text[startCut:endCut]
	if strings.HasSuffix(dropped, "\n") {
		omittedCount = strings.Count(dropped, "\n")
	} else {
		omittedCount = strings.Count(dropped, "\n") + 1
	}

	startPortion := text[:startCut]
	if startCut > 0 && startPortion[startCut-1] == '\n' {
		startPortion = startPortion[:startCut-1]
	}

	omittedStr := strconv.Itoa(omittedCount)
	result := startPortion + "\n[... truncated " + omittedStr + " lines ...]\n" + text[endCut:]

	// Verify we're under the limit (recursive safety)
	if EstimateTokenCount(result) > maxTokens {
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
		return TruncateToTokenLimit(result, maxTokens*90/100)
	}

	return result
}
