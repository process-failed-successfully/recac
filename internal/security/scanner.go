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
	reDangerousCmd    = regexp.MustCompile(`(?i)(?:^|[\s;&|()<>])(rm|cat|cp|mv|chmod|chown)\b.*(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	reRootDeletion    = regexp.MustCompile(`(?i)(?:^|[\s;&|()<>])rm\s+-[rRf]+\s+([/~*]+|/)(?:[\s;&|<>)]|$)`)
	rePipeShell       = regexp.MustCompile(`(?i)(?:^|[\s;&|()<>])(curl|wget)\b\s+.*?\|\s*(bash|sh|zsh|python|perl|php|ruby)\b`)
	reReverseShell    = regexp.MustCompile(`(?i)(?:^|[\s;&|()<>])nc\b\s+.*?-e\s+.*`)
)

// NewRegexScanner creates a new scanner with default patterns
func NewRegexScanner() *RegexScanner {
	return &RegexScanner{
		patterns: map[string]PatternConfig{
			"AWS Access Key":    {Pattern: reAWSAccessKey, IgnoreInQuotes: false},
			"Private Key":       {Pattern: rePrivateKey, IgnoreInQuotes: false},
			"Generic API Token": {Pattern: reGenericAPIToken, IgnoreInQuotes: false},
			"Slack Token":       {Pattern: reSlackToken, IgnoreInQuotes: false},
			"GitHub Token":      {Pattern: reGitHubToken, IgnoreInQuotes: false},
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

	maskedComments := maskContent(content, false)
	maskedAll := maskContent(content, true)

	for name, config := range s.patterns {
		contentToSearch := maskedComments
		if config.IgnoreInQuotes {
			contentToSearch = maskedAll
		}

		matches := config.Pattern.FindAllStringIndex(contentToSearch, -1)
		for _, match := range matches {
			// Find line number
			start := match[0]
			lineNumber := 1
			for i := 0; i < start; i++ {
				if contentToSearch[i] == '\n' {
					lineNumber++
				}
			}

			matchedText := contentToSearch[match[0]:match[1]]

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

func maskContent(content string, maskStrings bool) string {
	lines := strings.Split(content, "\n")
	maskedLines := make([]string, len(lines))
	for i, line := range lines {
		maskedLines[i] = maskLine(line, maskStrings)
	}
	return strings.Join(maskedLines, "\n")
}

func maskLine(line string, maskStrings bool) string {
	var sb strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(line); i++ {
		char := line[i]

		if escaped {
			escaped = false
			if maskStrings && (inSingleQuote || inDoubleQuote) {
				sb.WriteByte(' ')
			} else {
				sb.WriteByte(char)
			}
			continue
		}

		if char == '\\' {
			if !inSingleQuote {
				escaped = true
			}
			if maskStrings && inDoubleQuote {
				sb.WriteByte(' ')
			} else {
				sb.WriteByte(char)
			}
			continue
		}

		if char == '\'' {
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
				sb.WriteByte(char)
				continue
			}
		}

		if char == '"' {
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
				sb.WriteByte(char)
				continue
			}
		}

		// Check for comment
		if char == '#' && !inSingleQuote && !inDoubleQuote {
			isComment := false
			if i == 0 {
				isComment = true
			} else {
				prev := line[i-1]
				if prev == ' ' || prev == '\t' || prev == ';' || prev == '|' || prev == '&' || prev == '(' || prev == ')' || prev == '<' || prev == '>' {
					isComment = true
				}
			}

			if isComment {
				sb.WriteString(strings.Repeat(" ", len(line)-i))
				return sb.String()
			}
		}

		if maskStrings && (inSingleQuote || inDoubleQuote) {
			sb.WriteByte(' ')
		} else {
			sb.WriteByte(char)
		}
	}
	return sb.String()
}
