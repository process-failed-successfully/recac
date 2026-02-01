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
	reDangerousCmd    = regexp.MustCompile(`(?i)\b(rm|cat|cp|mv|chmod|chown)\b.*(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	reRootDeletion    = regexp.MustCompile(`(?i)\brm\s+-[rRf]+\s+([/~*]+|/)$`)
	rePipeShell       = regexp.MustCompile(`(?i)(curl|wget)\s+("[^"]*"|'[^']*'|\\\n|[^;&|\n])*?\|\s*(bash|sh|zsh|python|perl|php|ruby)`)
	reReverseShell    = regexp.MustCompile(`(?i)nc\s+("[^"]*"|'[^']*'|\\\n|[^;&|\n])*?-e\s+.*`)
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

// maskContent replaces comments (starting with #, and optionally //) and optionally strings with spaces
// to prevent false positives in regex matching, while preserving byte offsets.
func maskContent(content string, maskStrings bool, maskSlashComments bool) string {
	var sb strings.Builder
	sb.Grow(len(content))

	runes := []rune(content)
	n := len(runes)

	inString := false
	var stringChar rune
	inComment := false

	for i := 0; i < n; i++ {
		r := runes[i]
		rLen := utf8.RuneLen(r)
		spaces := strings.Repeat(" ", rLen)

		if inComment {
			if r == '\n' {
				inComment = false
				sb.WriteRune(r)
			} else {
				sb.WriteString(spaces)
			}
			continue
		}

		if inString {
			if r == stringChar {
				// Check for unescaped quote
				isEscaped := false
				if i > 0 && runes[i-1] == '\\' {
					// Count backslashes to handle \\"
					bsCount := 0
					for k := i - 1; k >= 0; k-- {
						if runes[k] == '\\' {
							bsCount++
						} else {
							break
						}
					}
					if bsCount%2 != 0 {
						isEscaped = true
					}
				}

				if !isEscaped {
					inString = false
				}
				sb.WriteString(spaces)
			} else {
				if r == '\n' {
					sb.WriteRune('\n')
				} else {
					sb.WriteString(spaces)
				}
			}
			continue
		}

		// Check for comment start
		if r == '#' {
			inComment = true
			sb.WriteString(spaces)
			continue
		}

		// Check for // comment
		if maskSlashComments && r == '/' && i+1 < n && runes[i+1] == '/' {
			inComment = true
			sb.WriteString(spaces)
			continue
		}

		// Check for string start
		if maskStrings && (r == '"' || r == '\'') {
			inString = true
			stringChar = r
			sb.WriteString(spaces)
			continue
		}

		sb.WriteRune(r)
	}

	return sb.String()
}

// Scan checks the content for security patterns
func (s *RegexScanner) Scan(content string) ([]Finding, error) {
	var findings []Finding

	// maskedShell: Strings masked, only # comments (preserve URLs with //)
	maskedShell := maskContent(content, true, false)
	// maskedCode: Strings masked, # and // comments masked
	maskedCode := maskContent(content, true, true)
	// maskedCommentsCode: Only # and // comments masked
	maskedCommentsCode := maskContent(content, false, true)

	for name, pattern := range s.patterns {
		var textToScan string
		switch name {
		case "Pipe to Shell":
			textToScan = maskedShell
		case "Reverse Shell":
			textToScan = maskedCode
		case "Dangerous Command", "Root Deletion":
			textToScan = maskedCommentsCode
		default:
			textToScan = content
		}

		matches := pattern.FindAllStringIndex(textToScan, -1)
		for _, match := range matches {
			// Find line number in the original content (textToScan has same newlines)
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
