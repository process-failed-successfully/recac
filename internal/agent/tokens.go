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

	// Optimization: Avoid strings.Split(text, "\n") which allocates O(L) memory where L is lines.
	// Instead, scan the string to find cut points.

	n := len(text)

	// Single line check or no newlines
	// Find first newline
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
		// Optimization: Instead of iterating line by line, find the last newline within the limit.
		// This is O(1) relative to line count (O(maxStartChars) scan, but optimized assembly).

		// We look for the last newline in the first maxStartChars bytes.
		// The character at maxStartChars-1 is the last one we can potentially include as a newline.
		// So we search in text[:maxStartChars].
		lastNL := strings.LastIndexByte(text[:maxStartChars], '\n')
		if lastNL != -1 {
			startCut = lastNL + 1
		} else {
			// No newline found in the allowed prefix.
			// This means the first line is longer than maxStartChars.
			// We can't include any full lines.
			startCut = 0
		}
	}

	// 2. Find end cut point
	endCut := n
	if maxEndChars >= n {
		endCut = 0
	} else {
		// Optimization: Find the first newline that allows the suffix to fit.
		// We want len(text[endCut:]) <= maxEndChars.
		// So endCut >= n - maxEndChars.
		// Also text[endCut-1] == '\n' (start of a line).

		// We search for the first newline at or after index (n - maxEndChars - 1).
		// If we find one at `idx`, then `endCut` can be `idx + 1`.
		// This ensures `endCut > n - maxEndChars - 1` => `endCut >= n - maxEndChars`.

		searchStart := n - maxEndChars - 1
		if searchStart < 0 {
			searchStart = 0
		}

		// Search in suffix starting at searchStart
		idx := strings.IndexByte(text[searchStart:], '\n')
		if idx != -1 {
			// Found a newline. The absolute index is searchStart + idx.
			// The line starts after this newline.
			endCut = searchStart + idx + 1
		} else {
			// No newline found in the suffix region.
			// This means we can't find a clean line break that fits the constraint.
			// We default to dropping everything (empty suffix).
			endCut = n
		}
	}

	// Check overlap
	if startCut >= endCut {
		return text
	}

	// Omitted lines count
	omittedCount := 0
	dropped := text[startCut:endCut]
	if dropped != "" {
		if strings.HasSuffix(dropped, "\n") {
			omittedCount = strings.Count(dropped, "\n")
		} else {
			omittedCount = strings.Count(dropped, "\n") + 1
		}
	}

	// Construct result.
	// startCut includes the trailing newline of the last start line.
	// truncationMarker starts with "\n".
	// This creates double newline "\n\n[... truncated ...]".
	// Original logic `strings.Join` didn't add trailing newline to start block.
	// So we should strip the last newline from start portion if we use the same marker.

	startPortion := text[:startCut]
	if startCut > 0 && startPortion[startCut-1] == '\n' {
		startPortion = startPortion[:startCut-1]
	}

	result := startPortion + "\n[... truncated " + strconv.Itoa(omittedCount) + " lines ...]\n" + text[endCut:]

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

	// Optimization: Avoid strings.Split(text, "\n\n") which allocates O(P) memory where P is paragraphs.
	// Instead, find paragraph boundaries using Index/LastIndex.
	sep := "\n\n"
	numParagraphs := strings.Count(text, sep) + 1

	if numParagraphs == 0 { // Should not happen if text is not empty, but safety check
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
	var firstPara string
	firstSepIdx := strings.Index(text, sep)
	if firstSepIdx == -1 {
		firstPara = text
	} else {
		firstPara = text[:firstSepIdx]
	}

	if EstimateTokenCount(firstPara) > tokensPerParagraph {
		firstPara = TruncateToTokenLimit(firstPara, tokensPerParagraph)
	}
	if firstPara != "" {
		summary.WriteString(firstPara)
		summary.WriteString("\n\n")
	}

	// Add middle summary if there are multiple paragraphs
	if numParagraphs > 2 {
		omittedCount := numParagraphs - 2
		summary.WriteString("[... " + strconv.Itoa(omittedCount) + " paragraphs omitted ...]\n\n")
	}

	// Add last paragraph (truncated if needed)
	if numParagraphs > 1 {
		var lastPara string
		lastSepIdx := strings.LastIndex(text, sep)
		if lastSepIdx != -1 {
			lastPara = text[lastSepIdx+len(sep):]
		} else {
			// Should not happen if numParagraphs > 1, but fallback
			lastPara = text
		}

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
