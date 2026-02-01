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

// PatternConfig defines configuration for a regex pattern
type PatternConfig struct {
	Pattern        *regexp.Regexp
	IgnoreInQuotes bool
}

// RegexScanner implements Scanner using regular expressions
type RegexScanner struct {
	patterns map[string]PatternConfig
}

var (
	reAWSAccessKey    = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	rePrivateKey      = regexp.MustCompile(`-----BEGIN [A-Z]+ PRIVATE KEY-----`)
	reGenericAPIToken = regexp.MustCompile(`(api|access)[_-]?key\s*[:=]\s*['"][a-zA-Z0-9_\-]{20,}['"]`)
	reSlackToken      = regexp.MustCompile(`xox[baprs]-([0-9a-zA-Z]{10,48})`)
	reGitHubToken     = regexp.MustCompile(`gh[pousr]_[a-zA-Z0-9]{36,255}`)
	reDangerousCmd    = regexp.MustCompile(`(?i)\b(rm|cat|cp|mv|chmod|chown)\b.*(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	reRootDeletion    = regexp.MustCompile(`(?i)\brm\s+-[rRf]+\s+([/~*]+|/)$`)
	rePipeShell       = regexp.MustCompile(`(?i)(curl|wget)\s+("[^"]*"|'[^']*'|\\\n|[^;&|\n])*?\|\s*(bash|sh|zsh|python|perl|php|ruby)`)
	reReverseShell    = regexp.MustCompile(`(?i)nc\s+("[^"]*"|'[^']*'|\\\n|[^;&|\n])*?-e\s+.*`)
)

// NewRegexScanner creates a new scanner with default patterns
func NewRegexScanner() *RegexScanner {
	return &RegexScanner{
		patterns: map[string]PatternConfig{
			"AWS Access Key":    {Pattern: reAWSAccessKey},
			"Private Key":       {Pattern: rePrivateKey},
			"Generic API Token": {Pattern: reGenericAPIToken},
			"Slack Token":       {Pattern: reSlackToken},
			"GitHub Token":      {Pattern: reGitHubToken},
			"Dangerous Command": {Pattern: reDangerousCmd, IgnoreInQuotes: true},
			"Root Deletion":     {Pattern: reRootDeletion, IgnoreInQuotes: true},
			"Pipe to Shell":     {Pattern: rePipeShell, IgnoreInQuotes: true},
			"Reverse Shell":     {Pattern: reReverseShell, IgnoreInQuotes: true},
		},
	}
}

// Scan checks the content for security patterns
func (s *RegexScanner) Scan(content string) ([]Finding, error) {
	var findings []Finding
	lines := strings.Split(content, "\n")
	maskedContent := maskContent(content)

	for name, config := range s.patterns {
		textToScan := content
		if config.IgnoreInQuotes {
			textToScan = maskedContent
		}

		matches := config.Pattern.FindAllStringIndex(textToScan, -1)
		for _, match := range matches {
			// Find line number
			start := match[0]
			lineNumber := 1
			for i := 0; i < start; i++ {
				if content[i] == '\n' {
					lineNumber++
				}
			}

			// Use original content for Match display
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

// maskContent replaces comments and quoted strings with spaces
// maintaining line numbers and total length
func maskContent(content string) string {
	var sb strings.Builder
	sb.Grow(len(content))

	inSingleQuote := false
	inDoubleQuote := false
	inComment := false
	escaped := false

	for i := 0; i < len(content); i++ {
		char := content[i]

		if inComment {
			if char == '\n' {
				sb.WriteByte(char)
				inComment = false
			} else {
				sb.WriteByte(' ')
			}
			continue
		}

		if inSingleQuote {
			if char == '\'' {
				inSingleQuote = false
				sb.WriteByte(char)
			} else {
				if char == '\n' {
					sb.WriteByte('\n')
				} else {
					sb.WriteByte(' ')
				}
			}
			continue
		}

		if inDoubleQuote {
			if escaped {
				escaped = false
				if char == '\n' {
					sb.WriteByte('\n')
				} else {
					sb.WriteByte(' ')
				}
				continue
			}
			if char == '\\' {
				escaped = true
				sb.WriteByte(' ')
				continue
			}
			if char == '"' {
				inDoubleQuote = false
				sb.WriteByte(char)
			} else {
				if char == '\n' {
					sb.WriteByte('\n')
				} else {
					sb.WriteByte(' ')
				}
			}
			continue
		}

		// Normal State
		if char == '#' {
			inComment = true
			sb.WriteByte(' ') // Mask the #
			continue
		}

		if char == '\'' {
			inSingleQuote = true
			sb.WriteByte(char)
			continue
		}

		if char == '"' {
			inDoubleQuote = true
			sb.WriteByte(char)
			continue
		}

		sb.WriteByte(char)
	}
	return sb.String()
}
