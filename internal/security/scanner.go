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
	reDangerousCmd    = regexp.MustCompile(`(?i)\b(rm|cat|cp|mv|chmod|chown)\b.*(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	reRootDeletion    = regexp.MustCompile(`(?im)\brm\s+-[rRf]+\s+([/~]+[/*.]*)(?:\s|;|$)`)
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
	maskedContent := maskComments(content)

	for name, pattern := range s.patterns {
		targetContent := content
		// For command-related checks, scan the masked content to ignore comments
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

			// We extract the matched text from the ORIGINAL content so the user sees what matched.
			// Since maskComments preserves length and indices, this is safe.
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

// maskComments replaces comments with spaces, preserving string length (byte-wise).
func maskComments(content string) string {
	b := []byte(content)
	masked := make([]byte, len(b))
	copy(masked, b)

	inSingleQuote := false
	inDoubleQuote := false
	inComment := false
	escaped := false
	wasSpace := true // Start of line counts as space

	for i := 0; i < len(b); i++ {
		c := b[i]

		if inComment {
			if c == '\n' {
				inComment = false
				// keep newline
				wasSpace = true
			} else {
				masked[i] = ' '
				// wasSpace irrelevant inside comment
			}
			continue
		}

		if inSingleQuote {
			if c == '\'' {
				inSingleQuote = false
			}
			wasSpace = false
			continue
		}

		if inDoubleQuote {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inDoubleQuote = false
			}
			wasSpace = false
			continue
		}

		// Outside quotes
		if escaped {
			escaped = false
			wasSpace = false
			continue
		}

		if c == '\\' {
			escaped = true
			wasSpace = false
			continue
		}

		if c == '\'' {
			inSingleQuote = true
			wasSpace = false
			continue
		}

		if c == '"' {
			inDoubleQuote = true
			wasSpace = false
			continue
		}

		if c == '#' && wasSpace {
			inComment = true
			masked[i] = ' '
			continue
		}

		// Update wasSpace
		if c == ' ' || c == '\t' || c == '\n' {
			wasSpace = true
		} else {
			wasSpace = false
		}
	}

	return string(masked)
}
