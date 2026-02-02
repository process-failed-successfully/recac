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

	// Masking Regexes
	reCBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	reCLineComment  = regexp.MustCompile(`//.*`)
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

// maskCComments replaces C-style comments with spaces, preserving line numbers, newlines, and byte length.
func maskCComments(content string) string {
	// 1. Block comments /* ... */
	content = reCBlockComment.ReplaceAllStringFunc(content, func(s string) string {
		// Replace with spaces, preserve newlines and byte length
		var sb strings.Builder
		sb.Grow(len(s))
		for _, r := range s {
			if r == '\n' {
				sb.WriteByte('\n')
			} else {
				n := utf8.RuneLen(r)
				for i := 0; i < n; i++ {
					sb.WriteByte(' ')
				}
			}
		}
		return sb.String()
	})

	// 2. Line comments // ...
	content = reCLineComment.ReplaceAllStringFunc(content, func(s string) string {
		return strings.Repeat(" ", len(s))
	})

	return content
}

// Scan checks the content for security patterns
func (s *RegexScanner) Scan(filename, content string) ([]Finding, error) {
	maskedContent := content
	ext := strings.ToLower(filepath.Ext(filename))
	cStyleExts := map[string]bool{
		".go": true, ".js": true, ".ts": true, ".java": true,
		".c": true, ".cpp": true, ".h": true, ".cs": true,
		".php": true, ".rs": true, ".swift": true, ".kt": true, ".scala": true,
	}

	if cStyleExts[ext] {
		maskedContent = maskCComments(content)
	}

	var findings []Finding
	lines := strings.Split(maskedContent, "\n")

	for name, pattern := range s.patterns {
		matches := pattern.FindAllStringIndex(maskedContent, -1)
		for _, match := range matches {
			// Find line number
			start := match[0]
			lineNumber := 1
			for i := 0; i < start; i++ {
				if maskedContent[i] == '\n' {
					lineNumber++
				}
			}

			matchedText := content[match[0]:match[1]] // Use original content for display

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
