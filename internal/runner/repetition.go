package runner

import (
	"strings"
)

// DetectRepetitiveLine checks if any single non-empty line repeats consecutively more than threshold times.
func DetectRepetitiveLine(lines []string, threshold int) (bool, int) {
	if len(lines) < threshold {
		return false, -1
	}

	count := 0
	lastLine := strings.TrimSpace(lines[0])
	if lastLine != "" {
		count = 1
	}

	for i := 1; i < len(lines); i++ {
		currentLine := strings.TrimSpace(lines[i])

		// Reset count if current line is empty (sequence must be non-empty lines)
		if currentLine == "" {
			count = 0
			lastLine = ""
			continue
		}

		if currentLine == lastLine {
			count++
			if count >= threshold {
				// We found 'threshold' repetitions ending at index i.
				// The start index is i - threshold + 1
				return true, i - threshold + 1
			}
		} else {
			count = 1
			lastLine = currentLine
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

// TruncateRepetitiveResponse checks for common repetition patterns and truncates the response if found.
func TruncateRepetitiveResponse(response string) (string, bool) {
	lines := strings.Split(response, "\n")

	// 1. Check for single line repeating 10 times
	if found, index := DetectRepetitiveLine(lines, 10); found {
		return strings.Join(lines[:index+1], "\n"), true
	}

	// 2. Check for 2-line pattern repeating 5 times
	if found, index := DetectRepetitiveSequence(lines, 2, 5); found {
		return strings.Join(lines[:index+2], "\n"), true
	}

	// 3. Check for 3-line pattern repeating 4 times
	if found, index := DetectRepetitiveSequence(lines, 3, 4); found {
		return strings.Join(lines[:index+3], "\n"), true
	}

	return response, false
}
