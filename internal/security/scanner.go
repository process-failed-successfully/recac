package security

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
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
	reDangerousCmd    = regexp.MustCompile(`(?i)(?:^|[\s;&|()<>])(rm|cat|cp|mv|chmod|chown)\b.*(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	reRootDeletion    = regexp.MustCompile(`(?i)(?:^|[\s;&|()<>])rm\s+-[rRf]+\s+([/~*]+|/)(?:[\s;&|<>)]|$)`)
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

	// Generate two masked versions:
	// 1. Strings masked (for dangerous commands to avoid false positives in echo/print)
	// 2. Strings preserved (for secrets which are often in strings)
	contentMaskedStrings := maskContent(content, true)
	contentPreservedStrings := maskContent(content, false)

	for name, pattern := range s.patterns {
		var targetContent string

		// Select the appropriate content version
		if name == "Dangerous Command" || name == "Root Deletion" {
			targetContent = contentMaskedStrings
		} else {
			targetContent = contentPreservedStrings
		}

		matches := pattern.FindAllStringIndex(targetContent, -1)
		for _, match := range matches {
			// Find line number
			start := match[0]
			lineNumber := 1
			for i := 0; i < start; i++ {
				if targetContent[i] == '\n' {
					lineNumber++
				}
			}

			matchedText := targetContent[match[0]:match[1]]

			findings = append(findings, Finding{
				Type:        name,
				Description: fmt.Sprintf("Found potential %s", name),
				Match:       matchedText,
				Line:        lineNumber,
			})
		}
	}

	// Scan line by line for context-aware checks (optional optimization)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Example: Check for hardcoded passwords in typical config patterns
		if strings.Contains(strings.ToLower(line), "password") && strings.Contains(line, "=") {
			// Very basic heuristic, improved by ensuring it's not a variable definition in code but a value assignment
			// For now, we'll be conservative to avoid noise, relying mostly on strict regexes above.
		}
		_ = i
	}

	return findings, nil
}

// maskContent replaces comments and optionally string literals with spaces.
// It preserves newlines and byte lengths to maintain correct line numbers and indices.
func maskContent(content string, maskStrings bool) string {
	var sb strings.Builder
	sb.Grow(len(content))

	inSingleQuote := false
	inDoubleQuote := false
	inComment := false
	escaped := false

	for i, r := range content {
		shouldMask := false

		if inComment {
			shouldMask = true
			if r == '\n' {
				inComment = false
				shouldMask = false
			}
		} else if inSingleQuote {
			if maskStrings {
				shouldMask = true
			}
			if r == '\'' && !escaped {
				inSingleQuote = false
				if maskStrings {
					shouldMask = false
				}
			}
		} else if inDoubleQuote {
			if maskStrings {
				shouldMask = true
			}
			if r == '"' && !escaped {
				inDoubleQuote = false
				if maskStrings {
					shouldMask = false
				}
			}
		} else {
			// Normal state
			if r == '#' {
				// Check if it's a valid comment start
				isComment := false
				if i == 0 {
					isComment = true
				} else {
					// Check previous byte
					prev := content[i-1]
					if prev == ' ' || prev == '\t' || prev == '\n' || prev == ';' || prev == '|' || prev == '&' || prev == '(' || prev == ')' || prev == '<' || prev == '>' {
						isComment = true
					}
				}

				if isComment {
					inComment = true
					shouldMask = true
				}
			} else if r == '\'' && !escaped {
				inSingleQuote = true
			} else if r == '"' && !escaped {
				inDoubleQuote = true
			}
		}

		// Handle escape state
		if r == '\\' && !inSingleQuote {
			escaped = !escaped
		} else {
			escaped = false
		}

		if shouldMask {
			if r == '\n' {
				sb.WriteRune(r)
			} else {
				l := utf8.RuneLen(r)
				for k := 0; k < l; k++ {
					sb.WriteByte(' ')
				}
			}
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
