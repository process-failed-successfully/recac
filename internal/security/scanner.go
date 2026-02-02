package security

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
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
	reDangerousCmd    = regexp.MustCompile(`(?i)\b(rm|cat|cp|mv|chmod|chown)\b.*(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	reRootDeletion    = regexp.MustCompile(`(?i)\brm\s+-[rRf]+\s+([/~*]+|/)$`)
	rePipeShell       = regexp.MustCompile(`(?i)(curl|wget)\s+.*?\|\s*(bash|sh|zsh|python|perl|php|ruby)`)
	reReverseShell    = regexp.MustCompile(`(?i)nc\s+.*?-e\s+.*`)
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

	// Create a masked version for context-aware checks (to avoid false positives in comments/strings)
	masked := maskContent(filename, content)

	for name, pattern := range s.patterns {
		// Use masked content for specific sensitive checks
		targetContent := content
		if name == "Dangerous Command" || name == "Root Deletion" || name == "Pipe to Shell" || name == "Reverse Shell" {
			targetContent = masked
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

			// Use original content for the match text
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

// maskContent replaces comments and quoted strings with whitespace/placeholders
// to prevent false positives in security scanning.
func maskContent(filename, content string) string {
	var sb strings.Builder
	sb.Grow(len(content))

	inDoubleQuote := false
	inSingleQuote := false
	inBacktick := false
	inLineComment := false // // or #

	// Determine comment style based on file extension
	maskSlashSlash := false
	maskHash := false

	ext := strings.ToLower(filepath.Ext(filename))
	base := strings.ToLower(filepath.Base(filename))

	// C-style comments (Go, JS, C, Java, etc.)
	switch ext {
	case ".go", ".js", ".ts", ".c", ".cpp", ".cc", ".h", ".hpp", ".java", ".scala", ".rs", ".kt":
		maskSlashSlash = true
	}

	// Hash comments (Shell, Python, Ruby, Perl, YAML, Dockerfile)
	switch ext {
	case ".sh", ".bash", ".zsh", ".py", ".rb", ".pl", ".yaml", ".yml", ".dockerfile":
		maskHash = true
	}
	if base == "dockerfile" || strings.HasSuffix(base, ".dockerfile") {
		maskHash = true
	}

	// If no explicit match, default to no masking to avoid false negatives?
	// Or maybe minimal masking?
	// For "test_spec" (binary), we shouldn't be scanning it anyway (filtered by size).
	// For "agent_command.sh" (dummy), maskHash should be true.

	runes := []rune(content)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		rLen := utf8.RuneLen(r)

		writePlaceholder := func(char byte) {
			for k := 0; k < rLen; k++ {
				sb.WriteByte(char)
			}
		}

		if inLineComment {
			if r == '\n' {
				inLineComment = false
				sb.WriteRune(r)
			} else {
				writePlaceholder(' ') // Replace comment content with space (preserve byte length)
			}
			continue
		}

		if inDoubleQuote {
			if r == '\\' {
				// Handle escaped characters (including escaped backslash)
				writePlaceholder('*') // mask \
				if i+1 < len(runes) {
					nextR := runes[i+1]
					nextLen := utf8.RuneLen(nextR)
					for k := 0; k < nextLen; k++ {
						sb.WriteByte('*')
					}
					i++
				}
				continue
			}
			if r == '"' {
				inDoubleQuote = false
				sb.WriteRune('"')
				continue
			}
			writePlaceholder('*') // Mask string content
			continue
		}

		if inSingleQuote {
			if r == '\\' {
				writePlaceholder('*')
				if i+1 < len(runes) {
					nextR := runes[i+1]
					nextLen := utf8.RuneLen(nextR)
					for k := 0; k < nextLen; k++ {
						sb.WriteByte('*')
					}
					i++
				}
				continue
			}
			if r == '\'' {
				inSingleQuote = false
				sb.WriteRune('\'')
				continue
			}
			writePlaceholder('*')
			continue
		}

		if inBacktick {
			if r == '`' {
				inBacktick = false
				sb.WriteRune('`')
			} else {
				writePlaceholder('*') // Mask string content
			}
			continue
		}

		// Detect start of string/comment
		if r == '"' {
			inDoubleQuote = true
			sb.WriteRune(r)
			continue
		}

		if r == '\'' {
			inSingleQuote = true
			sb.WriteRune(r)
			continue
		}

		if r == '`' {
			inBacktick = true
			sb.WriteRune(r)
			continue
		}

		// Comments: // or #
		// Check for //
		if maskSlashSlash && r == '/' && i+1 < len(runes) && runes[i+1] == '/' {
			// Heuristic: ignore // if preceded by : (e.g. http://)
			if i > 0 && runes[i-1] == ':' {
				sb.WriteRune(r)
				continue
			}

			inLineComment = true
			writePlaceholder(' ')

			// Handle next slash too
			nextR := runes[i+1]
			nextLen := utf8.RuneLen(nextR)
			for k := 0; k < nextLen; k++ {
				sb.WriteByte(' ')
			}
			i++
			continue
		}

		// Check for #
		if maskHash && r == '#' {
			inLineComment = true
			writePlaceholder(' ')
			continue
		}

		sb.WriteRune(r)
	}

	return sb.String()
}
