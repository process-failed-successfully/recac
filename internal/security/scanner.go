package security

import (
	"fmt"
	"regexp"
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
	reDangerousCmd    = regexp.MustCompile(`(?i)\b(rm|cat|cp|mv|chmod|chown)\b.*(?:^|[/\s"'])(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)(?:[/\s"']|$)`)
	// Use multiline mode (?m) to anchor to line ends, allow trailing whitespace
	reRootDeletion = regexp.MustCompile(`(?im)\brm\s+-[rRf]+\s+(/+\*?|~(/+\*?)?)\s*$`)
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

	// Mask only comments (for secret scanning)
	contentMaskedComments := maskComments(content)

	// Mask comments AND quotes (for command scanning)
	contentMaskedQuotes := maskCommentsAndQuotes(content)

	for name, pattern := range s.patterns {
		var targetContent string
		// Decide which content to use based on pattern name
		if name == "Dangerous Command" || name == "Root Deletion" {
			targetContent = contentMaskedQuotes
		} else {
			targetContent = contentMaskedComments
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

	return findings, nil
}

// maskCommentsAndQuotes replaces comments (inline and full line) and single-quoted string contents with spaces.
// Used for command scanning to avoid false positives in literals.
// Assumes Bash syntax (strict single quotes, escape supported in double quotes/normal).
func maskCommentsAndQuotes(content string) string {
	var sb strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	inComment := false
	escaped := false

	for i := 0; i < len(content); i++ {
		char := content[i]

		if char == '\n' {
			inSingleQuote = false
			inDoubleQuote = false
			inComment = false
			escaped = false
			sb.WriteByte('\n')
			continue
		}

		if inComment {
			sb.WriteByte(' ')
			continue
		}

		// Handle escapes (only if NOT in single quote for Bash)
		if !inSingleQuote && escaped {
			escaped = false
			sb.WriteByte(char)
			continue
		}

		if !inSingleQuote && char == '\\' {
			escaped = true
			sb.WriteByte('\\')
			continue
		}

		if inSingleQuote {
			if char == '\'' {
				inSingleQuote = false
				sb.WriteByte('\'')
			} else {
				sb.WriteByte(' ') // Mask content inside single quotes
			}
			continue
		}

		if inDoubleQuote {
			if char == '"' {
				inDoubleQuote = false
				sb.WriteByte('"')
			} else {
				sb.WriteByte(char) // Keep content inside double quotes
			}
			continue
		}

		// Normal mode
		if char == '\'' {
			inSingleQuote = true
			sb.WriteByte('\'')
		} else if char == '"' {
			inDoubleQuote = true
			sb.WriteByte('"')
		} else if char == '#' {
			inComment = true
			sb.WriteByte(' ')
		} else {
			sb.WriteByte(char)
		}
	}
	return sb.String()
}

// maskComments replaces comments with spaces, but PRESERVES quoted content.
// Used for secret scanning.
// Supports escapes in both single and double quotes (generic language support).
func maskComments(content string) string {
	var sb strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	inComment := false
	escaped := false

	for i := 0; i < len(content); i++ {
		char := content[i]

		if char == '\n' {
			inSingleQuote = false
			inDoubleQuote = false
			inComment = false
			escaped = false
			sb.WriteByte('\n')
			continue
		}

		if inComment {
			sb.WriteByte(' ')
			continue
		}

		if escaped {
			escaped = false
			sb.WriteByte(char) // Always keep escaped char content
			continue
		}

		if char == '\\' {
			escaped = true
			sb.WriteByte('\\')
			continue
		}

		if inSingleQuote {
			sb.WriteByte(char) // Keep content
			if char == '\'' {
				inSingleQuote = false
			}
			continue
		}

		if inDoubleQuote {
			sb.WriteByte(char) // Keep content
			if char == '"' {
				inDoubleQuote = false
			}
			continue
		}

		// Normal mode
		if char == '\'' {
			inSingleQuote = true
			sb.WriteByte('\'')
		} else if char == '"' {
			inDoubleQuote = true
			sb.WriteByte('"')
		} else if char == '#' {
			inComment = true
			sb.WriteByte(' ')
		} else {
			sb.WriteByte(char)
		}
	}
	return sb.String()
}
