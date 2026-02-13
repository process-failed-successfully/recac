package runner

import (
	"strings"
)

// DetectRepetitiveLine checks if any single non-empty line repeats consecutively more than threshold times.
func DetectRepetitiveLine(lines []string, threshold int) (bool, int) {
	trimmedLines := make([]string, len(lines))
	for i, l := range lines {
		trimmedLines[i] = strings.TrimSpace(l)
	}
	return detectRepetitiveLineTrimmed(trimmedLines, threshold)
}

func detectRepetitiveLineTrimmed(trimmedLines []string, threshold int) (bool, int) {
	if len(trimmedLines) < threshold {
		return false, -1
	}

	runStart := 0
	runCount := 0
	currentVal := ""

	for i, val := range trimmedLines {
		if val == "" {
			runCount = 0
			continue
		}

		if runCount == 0 {
			currentVal = val
			runStart = i
			runCount = 1
		} else if val == currentVal {
			runCount++
		} else {
			currentVal = val
			runStart = i
			runCount = 1
		}

		if runCount >= threshold {
			return true, runStart
		}
	}

	return false, -1
}

// DetectRepetitiveSequence checks if a pattern of K lines repeats R times.
func DetectRepetitiveSequence(lines []string, patternSize int, repeats int) (bool, int) {
	trimmedLines := make([]string, len(lines))
	for i, l := range lines {
		trimmedLines[i] = strings.TrimSpace(l)
	}
	return detectRepetitiveSequenceInternal(lines, trimmedLines, patternSize, repeats)
}

func detectRepetitiveSequenceInternal(lines []string, trimmedLines []string, patternSize int, repeats int) (bool, int) {
	totalNeeded := patternSize * repeats
	if len(lines) < totalNeeded {
		return false, -1
	}

	for i := 0; i <= len(lines)-totalNeeded; i++ {
		// Check for empty pattern using trimmedLines
		isPatternEmpty := true
		for j := 0; j < patternSize; j++ {
			if trimmedLines[i+j] != "" {
				isPatternEmpty = false
				break
			}
		}
		if isPatternEmpty {
			continue
		}

		// Define the pattern
		pattern := lines[i : i+patternSize]

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

	// Pre-compute trimmed lines once to avoid repeated allocations and operations
	trimmedLines := make([]string, len(lines))
	for i, l := range lines {
		trimmedLines[i] = strings.TrimSpace(l)
	}

	// 1. Check for single line repeating 10 times
	if found, index := detectRepetitiveLineTrimmed(trimmedLines, 10); found {
		return strings.Join(lines[:index+1], "\n"), true
	}

	// 2. Check for 2-line pattern repeating 5 times
	if found, index := detectRepetitiveSequenceInternal(lines, trimmedLines, 2, 5); found {
		return strings.Join(lines[:index+2], "\n"), true
	}

	// 3. Check for 3-line pattern repeating 4 times
	if found, index := detectRepetitiveSequenceInternal(lines, trimmedLines, 3, 4); found {
		return strings.Join(lines[:index+3], "\n"), true
	}

	return response, false
}
