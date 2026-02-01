package security

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
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
	reDangerousCmd    = regexp.MustCompile(`(?i)\b(rm|cat|cp|mv|chmod|chown)\b.*(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	reRootDeletion    = regexp.MustCompile(`(?i)\brm\s+-[rRf]+\s+([/~*]+|/)$`)
	rePipeShell       = regexp.MustCompile(`(?i)(curl|wget)\s+.*?\|\s*(bash|sh|zsh|python|perl|php|ruby)`)
	reReverseShell    = regexp.MustCompile(`(?i)nc\s+.*?-e\s+.*`)
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
			"Pipe to Shell":     rePipeShell,
			"Reverse Shell":     reReverseShell,
		},
	}
}

// Scan checks the content for security patterns
func (s *RegexScanner) Scan(content string) ([]Finding, error) {
	var findings []Finding
	lines := strings.Split(content, "\n")

	maskedContent := maskComments(content)

	for name, pattern := range s.patterns {
		targetContent := content
		// Code patterns that should ignore comments
		if name == "Pipe to Shell" || name == "Reverse Shell" || name == "Dangerous Command" || name == "Root Deletion" {
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

			// Extract from original content even if we matched on masked content
			// (Indices should align because maskComments preserves length)
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
			// Very basic heuristic, improved by ensuring it's not a variable definition in code but a value assignment
			// For now, we'll be conservative to avoid noise, relying mostly on strict regexes above.
		}
		_ = i
	}

	return findings, nil
}

// maskComments replaces comments with spaces, preserving original length and line offsets.
// It handles // and # style comments, respecting strings.
func maskComments(content string) string {
	var builder strings.Builder
	builder.Grow(len(content))

	runes := []rune(content)
	n := len(runes)

	inString := false
	var stringChar rune
	inLineComment := false

	for i := 0; i < n; i++ {
		r := runes[i]
		shouldMask := false

		if inLineComment {
			if r == '\n' {
				inLineComment = false
			} else {
				shouldMask = true
			}
		} else if inString {
			if r == stringChar {
				// Handle escape sequence
				escaped := false
				if i > 0 && runes[i-1] == '\\' {
					// Count consecutive backslashes
					backslashes := 0
					for j := i - 1; j >= 0; j-- {
						if runes[j] == '\\' {
							backslashes++
						} else {
							break
						}
					}
					if backslashes%2 != 0 {
						escaped = true
					}
				}
				if !escaped {
					inString = false
				}
			}
		} else {
			// Start of string?
			if r == '"' || r == '\'' || r == '`' {
				inString = true
				stringChar = r
			} else if r == '/' && i+1 < n && runes[i+1] == '/' {
				// Check for URL protocol (://)
				isURL := false
				if i > 0 && runes[i-1] == ':' {
					isURL = true
				}

				if !isURL {
					inLineComment = true
					shouldMask = true
				}
			} else if r == '#' {
				// Check bash comment
				isComment := false
				if i == 0 {
					isComment = true
				} else {
					if unicode.IsSpace(runes[i-1]) {
						isComment = true
					}
				}

				if isComment {
					inLineComment = true
					shouldMask = true
				}
			}
		}

		if shouldMask {
			// Replace with spaces equal to byte length to preserve indices
			len := utf8.RuneLen(r)
			for k := 0; k < len; k++ {
				builder.WriteByte(' ')
			}
		} else {
			builder.WriteRune(r)
		}
	}

	return builder.String()
}
