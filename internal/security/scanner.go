package security

import (
	"fmt"
	"regexp"
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

// PatternConfig defines a regex pattern and its scanning behavior
type PatternConfig struct {
	Name           string
	Pattern        *regexp.Regexp
	IgnoreInQuotes bool
}

// RegexScanner implements Scanner using regular expressions
type RegexScanner struct {
	patterns []PatternConfig
}

var (
	reAWSAccessKey    = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	rePrivateKey      = regexp.MustCompile(`-----BEGIN [A-Z]+ PRIVATE KEY-----`)
	reGenericAPIToken = regexp.MustCompile(`(api|access)[_-]?key\s*[:=]\s*['"][a-zA-Z0-9_\-]{20,}['"]`)
	reSlackToken      = regexp.MustCompile(`xox[baprs]-([0-9a-zA-Z]{10,48})`)
	reGitHubToken     = regexp.MustCompile(`gh[pousr]_[a-zA-Z0-9]{36,255}`)
	reDangerousCmd    = regexp.MustCompile(`(?i)(?:^|[\s;&|()<>])(rm|cat|cp|mv|chmod|chown)\b[^;&|\n]*?(?:^|[\s/])(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	reRootDeletion    = regexp.MustCompile(`(?i)(?:^|[\s;&|()<>])rm\s+-[rRf]+\s+(?:/|/\*|~)(?:[\s;&|<>)]|$)`)
	rePipeShell       = regexp.MustCompile(`(?i)(?:^|[\s;&|()<>])(curl|wget)\s+[^;&\n]*?\|\s*(bash|sh|zsh|python|perl|php|ruby)`)
	reReverseShell    = regexp.MustCompile(`(?i)(?:^|[\s;&|()<>])nc\s+[^;&|\n]*?-e\s+.*`)
)

// NewRegexScanner creates a new scanner with default patterns
func NewRegexScanner() *RegexScanner {
	return &RegexScanner{
		patterns: []PatternConfig{
			{Name: "AWS Access Key", Pattern: reAWSAccessKey, IgnoreInQuotes: false},
			{Name: "Private Key", Pattern: rePrivateKey, IgnoreInQuotes: false},
			{Name: "Generic API Token", Pattern: reGenericAPIToken, IgnoreInQuotes: false},
			{Name: "Slack Token", Pattern: reSlackToken, IgnoreInQuotes: false},
			{Name: "GitHub Token", Pattern: reGitHubToken, IgnoreInQuotes: false},
			// Dangerous commands should be ignored inside quotes (likely explanations)
			{Name: "Dangerous Command", Pattern: reDangerousCmd, IgnoreInQuotes: true},
			{Name: "Root Deletion", Pattern: reRootDeletion, IgnoreInQuotes: true},
			{Name: "Pipe to Shell", Pattern: rePipeShell, IgnoreInQuotes: true},
			{Name: "Reverse Shell", Pattern: reReverseShell, IgnoreInQuotes: true},
		},
	}
}

// Scan checks the content for security patterns
func (s *RegexScanner) Scan(content string) ([]Finding, error) {
	var findings []Finding

	// Generate masked versions once
	maskedCommentsOnly := maskContent(content, false)
	maskedQuotesAndComments := maskContent(content, true)

	for _, config := range s.patterns {
		var targetContent string
		if config.IgnoreInQuotes {
			targetContent = maskedQuotesAndComments
		} else {
			targetContent = maskedCommentsOnly
		}

		matches := config.Pattern.FindAllStringIndex(targetContent, -1)
		for _, match := range matches {
			// Find line number
			start := match[0]
			lineNumber := 1
			for i := 0; i < start; i++ {
				if targetContent[i] == '\n' {
					lineNumber++
				}
			}

			// Use original content for the match text so it's readable
			matchedText := content[match[0]:match[1]]

			findings = append(findings, Finding{
				Type:        config.Name,
				Description: fmt.Sprintf("Found potential %s", config.Name),
				Match:       matchedText,
				Line:        lineNumber,
			})
		}
	}

	return findings, nil
}

// maskContent replaces comments and optionally quoted strings with spaces using byte-level iteration.
// maskQuotes: if true, content inside quotes is also masked.
func maskContent(content string, maskQuotes bool) string {
	bytes := []byte(content)
	n := len(bytes)

	inSingleQuote := false
	inDoubleQuote := false
	escaped := false
	inComment := false

	for i := 0; i < n; i++ {
		b := bytes[i]

		// Reset state on newline
		if b == '\n' {
			inComment = false
			escaped = false
			continue
		}

		if inComment {
			bytes[i] = ' '
			continue
		}

		if escaped {
			// If inside quote, we are processing content
			if inSingleQuote || inDoubleQuote {
				if maskQuotes {
					bytes[i] = ' '
				}
			}
			escaped = false
			continue
		}

		if b == '\\' {
			if inSingleQuote {
				// Backslash is literal in single quotes
				if maskQuotes {
					bytes[i] = ' '
				}
			} else {
				// Start escape sequence
				escaped = true
				if inDoubleQuote {
					if maskQuotes {
						bytes[i] = ' ' // Mask the backslash itself if inside double quote
					}
				}
			}
			continue
		}

		if b == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}

		if b == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		if inSingleQuote || inDoubleQuote {
			if maskQuotes {
				bytes[i] = ' '
			}
			continue
		}

		// Check for comment start
		if b == '#' {
			isComment := false
			if i == 0 {
				isComment = true
			} else {
				prev := bytes[i-1]
				// Bash comment start conditions
				if prev == ' ' || prev == '\t' || prev == ';' || prev == '|' || prev == '&' || prev == '(' || prev == ')' || prev == '<' || prev == '>' {
					isComment = true
				}
			}

			if isComment {
				inComment = true
				bytes[i] = ' '
			}
		}
	}
	return string(bytes)
}
