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

// PatternConfig defines configuration for a specific regex pattern
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
	// Note: rePipeShell and reReverseShell patterns are robust but we also rely on IgnoreInQuotes for extra safety
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
			"Dangerous Command": {Pattern: reDangerousCmd, IgnoreInQuotes: false},
			"Root Deletion":     {Pattern: reRootDeletion, IgnoreInQuotes: false},
			"Pipe to Shell":     {Pattern: rePipeShell, IgnoreInQuotes: true},
			"Reverse Shell":     {Pattern: reReverseShell, IgnoreInQuotes: true},
		},
	}
}

// Scan checks the content for security patterns
func (s *RegexScanner) Scan(content string) ([]Finding, error) {
	var findings []Finding
	lines := strings.Split(content, "\n")

	// Create masked content once if needed, or on demand.
	// Since we might need it for multiple patterns, we could compute it lazily.
	var maskedContent string
	maskedContentInitialized := false

	for name, config := range s.patterns {
		textToScan := content
		if config.IgnoreInQuotes {
			if !maskedContentInitialized {
				maskedContent = maskContent(content)
				maskedContentInitialized = true
			}
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

			// We use the original content for the matched text display
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
			// Very basic heuristic, preserved from original
		}
		_ = i
	}

	return findings, nil
}

// maskContent replaces comments and quoted strings with spaces
// maintaining the original byte length and line breaks.
func maskContent(content string) string {
	// We work with bytes to ensure we preserve exact length for regex index matching
	bytes := []byte(content)
	n := len(bytes)

	inSingleQuote := false
	inDoubleQuote := false
	inComment := false
	escaped := false

	// Iterate byte by byte.
	// This is safe for UTF-8 as long as we only trigger on ASCII delimiters (#, ', ", \, \n)
	// which are single bytes in UTF-8 and cannot appear as part of a multi-byte sequence.
	for i := 0; i < n; i++ {
		c := bytes[i]

		if inComment {
			if c == '\n' {
				inComment = false
			} else {
				bytes[i] = ' '
			}
			continue
		}

		if inSingleQuote {
			if c == '\'' {
				// Strict single quotes in bash/sh: no escaping allowed.
				// The string ends at the next single quote.
				inSingleQuote = false
				bytes[i] = ' '
			} else {
				bytes[i] = ' '
			}
			continue
		}

		if inDoubleQuote {
			if escaped {
				escaped = false
				bytes[i] = ' ' // Mask the escaped char
				continue
			}

			if c == '\\' {
				escaped = true
				bytes[i] = ' '
				continue
			}

			if c == '"' {
				inDoubleQuote = false
				bytes[i] = ' '
			} else {
				bytes[i] = ' '
			}
			continue
		}

		// Outside quotes/comments
		if escaped {
			escaped = false
			// We don't mask the escaped char itself in the general case unless it's part of an operator we want to hide?
			// But for safety, we just skip it so it doesn't trigger start of quotes/comments.
			continue
		}

		if c == '\\' {
			escaped = true
			continue
		}

		switch c {
		case '#':
			// Comment must be at start of line or preceded by separator
			isComment := false
			if i == 0 {
				isComment = true
			} else {
				prev := bytes[i-1]
				switch prev {
				case ' ', '\t', '\n', ';', '|', '&', '(', ')':
					isComment = true
				}
			}

			if isComment {
				inComment = true
				bytes[i] = ' '
			}
		case '\'':
			inSingleQuote = true
			bytes[i] = ' '
		case '"':
			inDoubleQuote = true
			bytes[i] = ' '
		}
	}

	return string(bytes)
}
