package security

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Scanner defines the interface for security scanning
type Scanner interface {
	Scan(filename, content string) ([]Finding, error)
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
	reRootDeletion = regexp.MustCompile(`(?im)(?:^|[\s;&|()<>` + "`" + `\\])rm\s+-[rRf]+\s+(/\*?|~(/+\*?)?)\s*$`)
	rePipeShell    = regexp.MustCompile(`(?i)\b(curl|wget)\s+.*?\|\s*(bash|sh|zsh|python|perl|php|ruby)\b`)
	reReverseShell = regexp.MustCompile(`(?i)\bnc\s+.*?-e\s+.*`)
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
func (s *RegexScanner) Scan(filename, content string) ([]Finding, error) {
	var findings []Finding
	lines := strings.Split(content, "\n")

	// Determine masking strategy based on file extension
	var maskedContent string
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".go", ".js", ".ts", ".java", ".c", ".cpp", ".h", ".cs", ".php":
		maskedContent = maskCComments(content)
	case ".sh", ".py", ".rb", ".yaml", ".yml", ".dockerfile":
		maskedContent = maskShellComments(content)
	default:
		// Check for specific filenames (like Dockerfile with no ext)
		if strings.HasSuffix(filename, "Dockerfile") {
			maskedContent = maskShellComments(content)
		} else {
			// Default to shell-style comments as it's most common for scripts
			// or maybe no masking? Let's use shell-style as safe default for scripts.
			maskedContent = maskShellComments(content)
		}
	}

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
		if name == "Dangerous Command" || name == "Root Deletion" || name == "Pipe to Shell" || name == "Reverse Shell" {
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

// maskShellComments replaces comments in Bash/Python scripts with spaces, preserving layout.
// It handles strings (single/double quotes) to avoid masking '#' inside them.
func maskShellComments(content string) string {
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

// maskCComments replaces C-style comments (// and /* */) with spaces.
func maskCComments(content string) string {
	var masked []byte
	masked = make([]byte, len(content))
	copy(masked, content)

	inSingle := false
	inDouble := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(content); i++ {
		char := content[i]

		if inLineComment {
			if char == '\n' {
				inLineComment = false
			} else {
				masked[i] = ' '
			}
			continue
		}

		if inBlockComment {
			if char == '*' && i+1 < len(content) && content[i+1] == '/' {
				inBlockComment = false
				masked[i] = ' '
				masked[i+1] = ' '
				i++ // Skip /
			} else if char != '\n' {
				masked[i] = ' '
			}
			continue
		}

		if inSingle {
			if char == '\'' && (i == 0 || content[i-1] != '\\') {
				inSingle = false
			}
			continue
		}

		if inDouble {
			if char == '"' && (i == 0 || content[i-1] != '\\') {
				inDouble = false
			}
			continue
		}

		// Check for comment start
		if i+1 < len(content) && char == '/' {
			if content[i+1] == '/' {
				inLineComment = true
				masked[i] = ' '
				masked[i+1] = ' '
				i++
				continue
			} else if content[i+1] == '*' {
				inBlockComment = true
				masked[i] = ' '
				masked[i+1] = ' '
				i++
				continue
			}
		}

		if char == '"' {
			inDouble = true
		} else if char == '\'' {
			inSingle = true
		}
	}

	return string(masked)
}
