package agent

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
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

	truncationMarker := "\n[... truncated ...]\n"
	markerTokens := EstimateTokenCount(truncationMarker)

	// Use a shorter marker if we are very constrained
	// If marker takes more than 1/3 of available space, switch to shorter marker
	useShortMarker := false
	if markerTokens > maxTokens/3 {
		truncationMarker = "..."
		markerTokens = EstimateTokenCount(truncationMarker)
		useShortMarker = true
	}

	availableTokens := maxTokens - markerTokens
	if availableTokens <= 0 {
		return ""
	}

	maxChars := availableTokens * 4
	halfChars := maxChars / 2

	// Start portion
	startCut := halfChars
	if startCut > len(text) {
		startCut = len(text)
	} else {
		// Ensure we don't split a rune
		for startCut > 0 && !utf8.RuneStart(text[startCut]) {
			startCut--
		}
	}

	// Try to cut at newline if close (within 100 chars)
	if idx := strings.LastIndexByte(text[:startCut], '\n'); idx != -1 && startCut-idx < 100 {
		startCut = idx + 1 // Include newline
	}

	// End portion
	endCut := len(text) - halfChars
	if endCut < startCut {
		endCut = startCut
	} else {
		// Ensure we don't split a rune
		for endCut < len(text) && !utf8.RuneStart(text[endCut]) {
			endCut++
		}
	}

	// Try to cut at newline if close
	if idx := strings.IndexByte(text[endCut:], '\n'); idx != -1 && idx < 100 {
		endCut += idx + 1 // Start after newline
	}

	if startCut >= endCut {
		return text
	}

	omittedCount := strings.Count(text[startCut:endCut], "\n")
	var marker string
	if useShortMarker {
		marker = truncationMarker
	} else {
		marker = fmt.Sprintf("\n[... truncated %d lines ...]\n", omittedCount)
		if omittedCount == 0 {
			marker = truncationMarker
		}
	}

	result := text[:startCut] + marker + text[endCut:]

	// Verification and recursion
	if EstimateTokenCount(result) > maxTokens {
		return TruncateToTokenLimit(result, maxTokens*90/100)
	}
	return result
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
