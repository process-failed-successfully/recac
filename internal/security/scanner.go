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

// PatternConfig defines configuration for a specific security pattern
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

	// Command matching patterns to avoid false positives (e.g. echo "rm -rf /")
	// We require the command to be at the start of a line, or preceded by a separator/grouper, or a known command runner.
	// We use \W+ to handle spaces, quotes, parens etc. after the runner.
	cmdPrefix      = `(?:^|[;&|{(]|\b(?:sudo|xargs|time|nice|nohup|watch|env|start|exec|eval)\W+)\s*`
	reDangerousCmd = regexp.MustCompile(`(?mi)` + cmdPrefix + `\b(rm|cat|cp|mv|chmod|chown)\b.*(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	// reRootDeletion must ensure it matches the specific path and not a prefix (e.g. /tmp), but allow trailing separators/quotes.
	reRootDeletion = regexp.MustCompile(`(?mi)` + cmdPrefix + `\brm\s+-[rRf]+\s+([/~]+|/|/\*)(?:$|[\s;&|)'"])`)

	// New patterns
	// Note: We use cmdPrefix here to avoid false positives in documentation/strings (e.g. echo "curl | bash")
	// while still catching executed commands (e.g. eval "curl | bash").
	rePipeShell    = regexp.MustCompile(`(?mi)` + cmdPrefix + `(curl|wget)\s+.*?\|\s*(bash|sh|zsh|python|perl|php|ruby)`)
	reReverseShell = regexp.MustCompile(`(?mi)` + cmdPrefix + `nc\s+.*?-e\s+.*`)
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
			// We disable IgnoreInQuotes for commands because we MUST catch commands inside 'eval', 'bash -c', etc.
			// The cmdPrefix regex handles false positives (like 'echo') correctly.
			"Dangerous Command": {Pattern: reDangerousCmd, IgnoreInQuotes: false},
			"Root Deletion":     {Pattern: reRootDeletion, IgnoreInQuotes: false},
			"Pipe to Shell":     {Pattern: rePipeShell, IgnoreInQuotes: false},
			"Reverse Shell":     {Pattern: reReverseShell, IgnoreInQuotes: false},
		},
	}
}

// maskContent replaces comments and optionally quoted content with spaces.
// It uses a stateful parser to handle quotes and comments correctly.
func (s *RegexScanner) maskContent(content string, maskQuotes bool) string {
	var sb strings.Builder
	sb.Grow(len(content))

	inSingleQuote := false
	inDoubleQuote := false
	escaped := false
	inComment := false // Starts with #

	for i := 0; i < len(content); i++ {
		char := content[i]

		// Handle state transitions
		if inComment {
			if char == '\n' {
				inComment = false
				sb.WriteByte(char)
			} else {
				sb.WriteByte(' ')
			}
			continue
		}

		if inSingleQuote {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '\'' {
				inSingleQuote = false
			}
			if maskQuotes {
				if char == '\n' {
					sb.WriteByte(char)
				} else {
					sb.WriteByte(' ')
				}
			} else {
				sb.WriteByte(char)
			}
			continue
		}

		if inDoubleQuote {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inDoubleQuote = false
			}
			if maskQuotes {
				if char == '\n' {
					sb.WriteByte(char)
				} else {
					sb.WriteByte(' ')
				}
			} else {
				sb.WriteByte(char)
			}
			continue
		}

		// Normal state
		if char == '#' {
			inComment = true
			sb.WriteByte(' ') // Mask the # itself
		} else if char == '\'' {
			inSingleQuote = true
			if maskQuotes {
				sb.WriteByte(' ')
			} else {
				sb.WriteByte(char)
			}
		} else if char == '"' {
			inDoubleQuote = true
			if maskQuotes {
				sb.WriteByte(' ')
			} else {
				sb.WriteByte(char)
			}
		} else {
			sb.WriteByte(char)
		}
	}
	return sb.String()
}

// Scan checks the content for security patterns
func (s *RegexScanner) Scan(content string) ([]Finding, error) {
	// Pre-calculate masked contents
	maskedWithQuotes := s.maskContent(content, true)
	maskedWithoutQuotes := s.maskContent(content, false)

	var findings []Finding

	for name, config := range s.patterns {
		var contentToScan string
		if config.IgnoreInQuotes {
			contentToScan = maskedWithQuotes
		} else {
			contentToScan = maskedWithoutQuotes
		}

		matches := config.Pattern.FindAllStringIndex(contentToScan, -1)
		for _, match := range matches {
			// Find line number using original content
			start := match[0]
			lineNumber := 1
			for i := 0; i < start; i++ {
				if content[i] == '\n' {
					lineNumber++
				}
			}

			// Extract matched text from ORIGINAL content
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
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Example: Check for hardcoded passwords in typical config patterns
		if strings.Contains(strings.ToLower(line), "password") && strings.Contains(line, "=") {
			// Very basic heuristic
		}
		_ = i
	}

	return findings, nil
}
