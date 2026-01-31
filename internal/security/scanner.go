package security

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Scanner defines the interface for security scanning
type Scanner interface {
	Scan(content string) ([]Finding, error)
}

// Finding represents a security issue found in the content
type Finding struct {
	Type        string
	Description string
	Match       string
	Line        int
}

// RegexScanner implements Scanner using regular expressions
type RegexScanner struct {
	patterns map[string]*regexp.Regexp
}

var (
	reAWSAccessKey    = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	rePrivateKey      = regexp.MustCompile(`-----BEGIN [A-Z]+ PRIVATE KEY-----`)
	reGenericAPIToken = regexp.MustCompile(`(api|access)[_-]?key\s*[:=]\s*['"][a-zA-Z0-9_\-]{20,}['"]`)
	reSlackToken      = regexp.MustCompile(`xox[baprs]-([0-9a-zA-Z]{10,48})`)
	reGitHubToken     = regexp.MustCompile(`gh[pousr]_[a-zA-Z0-9]{36,255}`)
	reDangerousCmd    = regexp.MustCompile(`(?i)\b(rm|cat|cp|mv|chmod|chown)\b.*(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	reRootDeletion    = regexp.MustCompile(`(?i)\brm\s+-[rRf]+\s+([/~*]+|/)$`)
)

// NewRegexScanner creates a new scanner with default patterns
func NewRegexScanner() *RegexScanner {
	return &RegexScanner{
		patterns: map[string]*regexp.Regexp{
			"AWS Access Key":    reAWSAccessKey,
			"Private Key":       rePrivateKey,
			"Generic API Token": reGenericAPIToken,
			"Slack Token":       reSlackToken,
			"GitHub Token":      reGitHubToken,
			"Dangerous Command": reDangerousCmd,
			"Root Deletion":     reRootDeletion,
		},
	}
}

// Scan checks the content for security patterns
func (s *RegexScanner) Scan(content string) ([]Finding, error) {
	var findings []Finding
	lines := strings.Split(content, "\n")

	// Pre-calculate masked content for command checks
	maskedContent := maskComments(content)

	// Sort keys for deterministic iteration order
	keys := make([]string, 0, len(s.patterns))
	for k := range s.patterns {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		pattern := s.patterns[name]

		// Use masked content for command checks, original for secrets
		targetContent := content
		if name == "Dangerous Command" || name == "Root Deletion" {
			targetContent = maskedContent
		}

		matches := pattern.FindAllStringIndex(targetContent, -1)
		for _, match := range matches {
			// Find line number
			start := match[0]
			lineNumber := 1
			for i := 0; i < start; i++ {
				if content[i] == '\n' {
					lineNumber++
				}
			}

			// Use original content for the match text to show what was found
			// (even if we matched on masked content, the text corresponds to the command)
			matchedText := content[match[0]:match[1]]

			findings = append(findings, Finding{
				Type:        name,
				Description: fmt.Sprintf("Found potential %s", name),
				Match:       matchedText,
				Line:        lineNumber,
			})
		}
	}

	// Scan line by line for context-aware checks (optional optimization)
	for i, line := range lines {
		// Example: Check for hardcoded passwords in typical config patterns
		if strings.Contains(strings.ToLower(line), "password") && strings.Contains(line, "=") {
			// Very basic heuristic
		}
		_ = i
	}

	return findings, nil
}

// maskComments replaces Bash-style comments with spaces to avoid false positives in command scanning.
// It preserves string length and line numbers.
func maskComments(content string) string {
	lines := strings.Split(content, "\n")
	var maskedLines []string

	for _, line := range lines {
		idx := findCommentStart(line)
		if idx != -1 {
			// Convert to bytes to modify in place (conceptually) and handle length correctly
			// We replace comment chars with spaces
			lineBytes := []byte(line)
			for i := idx; i < len(lineBytes); i++ {
				lineBytes[i] = ' '
			}
			maskedLines = append(maskedLines, string(lineBytes))
		} else {
			maskedLines = append(maskedLines, line)
		}
	}
	// Rejoin with newline. Note: this normalizes line endings to \n if they were different.
	// But since we split by \n, we assume \n is the separator.
	return strings.Join(maskedLines, "\n")
}

// findCommentStart returns the index of the start of a comment (#), or -1 if none.
// It handles single/double quotes and ensures # is preceded by a delimiter.
func findCommentStart(line string) int {
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(line); i++ {
		char := line[i]

		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}

		if char == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}

		if !inSingle && !inDouble {
			if char == '#' {
				// Check if it's a valid comment start
				if i == 0 {
					return i
				}
				prev := line[i-1]
				// Check if prev is a delimiter
				if isDelimiter(prev) {
					return i
				}
			}
		}
	}
	return -1
}

func isDelimiter(b byte) bool {
	// Space, tab
	if b == ' ' || b == '\t' {
		return true
	}
	// Bash control operators
	switch b {
	case ';', '|', '&', '(', ')', '<', '>':
		return true
	}
	return false
}
