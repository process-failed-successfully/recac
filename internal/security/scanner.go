package security

import (
	"fmt"
	"regexp"
	"sort"
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
	// Updated regexes with stricter boundaries and better string/comment handling awareness
	// Note: We include '\\' in the boundary to catch escaped commands like '\rm'
	reDangerousCmd = regexp.MustCompile(`(?i)(?:^|[\s;&|()<>` + "`" + `\\])(rm|cat|cp|mv|chmod|chown)\b.*(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	reRootDeletion = regexp.MustCompile(`(?im)(?:^|[\s;&|()<>` + "`" + `\\])rm\s+-[rRf]+\s+([/~*]+|/)\s*$`)
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

	// Pre-process content for command checks (mask comments)
	maskedContent := maskComments(content)

	// Get keys and sort them for deterministic order
	keys := make([]string, 0, len(s.patterns))
	for k := range s.patterns {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		pattern := s.patterns[name]

		// For sensitive data (Secrets), scan the ORIGINAL content (leaked secrets in comments are still leaks).
		// For command validation (Dangerous Command), scan the MASKED content (commented commands are safe).
		targetContent := content
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

			// Extract matched text from ORIGINAL content to show what was found
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

// maskComments replaces comments in Bash scripts with spaces, preserving layout.
// It handles strings (single/double quotes) to avoid masking '#' inside them.
func maskComments(content string) string {
	var masked []byte
	// Convert to byte slice for mutability
	masked = make([]byte, len(content))
	copy(masked, content)

	inSingle := false
	inDouble := false
	inComment := false

	for i := 0; i < len(content); i++ {
		char := content[i]

		if inComment {
			if char == '\n' {
				inComment = false
				// Newline is preserved
			} else {
				masked[i] = ' '
			}
			continue
		}

		if inSingle {
			if char == '\'' {
				inSingle = false
			}
			continue
		}

		if inDouble {
			if char == '"' {
				// Handle escaped quote
				if i > 0 && content[i-1] == '\\' {
					// check for double escape (backslash itself escaped)
					// e.g. "foo\\" -> escape, "foo\\\"" -> quote escaped
					escaped := false
					// count preceding backslashes
					bsCount := 0
					for j := i - 1; j >= 0; j-- {
						if content[j] == '\\' {
							bsCount++
						} else {
							break
						}
					}
					if bsCount%2 != 0 {
						escaped = true
					}

					if !escaped {
						inDouble = false
					}
				} else {
					inDouble = false
				}
			}
			continue
		}

		// Not in string or comment
		if char == '#' {
			// Check if it's a valid comment start
			// Must be at start of line OR preceded by whitespace/control operator
			isStart := false
			if i == 0 {
				isStart = true
			} else {
				prev := content[i-1]
				switch prev {
				case ' ', '\t', '\n', ';', '|', '&', '(', ')', '<', '>', '`':
					isStart = true
				}
			}

			if isStart {
				inComment = true
				masked[i] = ' '
			}
		} else if char == '\'' {
			inSingle = true
		} else if char == '"' {
			inDouble = true
		}
	}

	return string(masked)
}
