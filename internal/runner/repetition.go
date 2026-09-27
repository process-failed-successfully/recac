package runner

import (
	"strings"
)

// DetectRepetitiveLine checks if any single non-empty line repeats consecutively more than threshold times.
func DetectRepetitiveLine(lines []string, threshold int) (bool, int) {
	if len(lines) < threshold {
		return false, -1
	}

	for i := 0; i <= len(lines)-threshold; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		repeated := true
		for j := 1; j < threshold; j++ {
			if strings.TrimSpace(lines[i+j]) != line {
				repeated = false
				break
			}
		}
		if repeated {
			return true, i
		}
	}
	return false, -1
}

// DetectRepetitiveSequence checks if a pattern of K lines repeats R times.
func DetectRepetitiveSequence(lines []string, patternSize int, repeats int) (bool, int) {
	totalNeeded := patternSize * repeats
	if len(lines) < totalNeeded {
		return false, -1
	}

	for i := 0; i <= len(lines)-totalNeeded; i++ {
		// Define the pattern
		pattern := lines[i : i+patternSize]

		isPatternEmpty := true
		for _, pl := range pattern {
			if strings.TrimSpace(pl) != "" {
				isPatternEmpty = false
				break
			}
		}
		if isPatternEmpty {
			continue
		}

		allMatch := true
		for r := 1; r < repeats; r++ {
			start := i + (r * patternSize)
			for p := 0; p < patternSize; p++ {
				if lines[start+p] != pattern[p] {
					allMatch = false
					break
				}
			}
			if !allMatch {
				break
			}
		}

		if allMatch {
			return true, i
		}
	}
	return false, -1
}

// extractTruncatedLines returns the first `numLines` from `response` separated by '\n'.
func extractTruncatedLines(response string, numLines int) string {
	if numLines <= 0 {
		return ""
	}

	count := 0
	for i := 0; i < len(response); i++ {
		if response[i] == '\n' {
			count++
			if count == numLines {
				return response[:i]
			}
		}
	}
	return response
}

// TruncateRepetitiveResponse checks for common repetition patterns and truncates the response if found.
func TruncateRepetitiveResponse(response string) (string, bool) {
	// ⚡ Bolt: Fast return if the response is too short to have repetitions
	if len(response) < 10 {
		return response, false
	}

	count := strings.Count(response, "\n") + 1
	if count < 10 {
		return response, false
	}

	// ⚡ Bolt: Avoid strings.Split intermediate allocations by extracting substrings
	lines := make([]string, 0, count)
	outStr := response
	for {
		idx := strings.IndexByte(outStr, '\n')
		if idx == -1 {
			lines = append(lines, outStr)
			break
		}
		lines = append(lines, outStr[:idx])
		outStr = outStr[idx+1:]
	}

	// 1. Check for single line repeating 10 times
	if found, index := DetectRepetitiveLine(lines, 10); found {
		return extractTruncatedLines(response, index+1), true
	}

	// 2. Check for 2-line pattern repeating 5 times
	if found, index := DetectRepetitiveSequence(lines, 2, 5); found {
		return extractTruncatedLines(response, index+2), true
	}

	// 3. Check for 3-line pattern repeating 4 times
	if found, index := DetectRepetitiveSequence(lines, 3, 4); found {
		return extractTruncatedLines(response, index+3), true
	}

	return response, false
}
