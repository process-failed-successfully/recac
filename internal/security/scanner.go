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

	// Boundary for commands: Start of line or shell delimiters.
	// We deliberately exclude quotes '"' so that matches inside strings are ignored.
	boundary = `(?:^|[\s;&|()<>` + "`" + `])`

	reDangerousCmd = regexp.MustCompile(`(?i)` + boundary + `(rm|cat|cp|mv|chmod|chown)\b.*(?:^|[/\s"'])(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	// Allow trailing whitespace (\s*) because masking replaces comments with spaces
	reRootDeletion = regexp.MustCompile(`(?i)` + boundary + `rm\s+-[rRf]+\s+(/+\*?|~(/+\*?)?)\s*$`)
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

// Scan checks the content for security patterns line by line
func (s *RegexScanner) Scan(content string) ([]Finding, error) {
	var findings []Finding
	lines := strings.Split(content, "\n")

	// Sort patterns for deterministic order
	keys := make([]string, 0, len(s.patterns))
	for k := range s.patterns {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, line := range lines {
		// Mask comments for command scanning
		maskedLine := maskComments(line)

		// Optimization: if masked line is empty/blank, we can skip command checks
		// But secrets might still be there (though unlikely in comments? Secrets in comments are leaks too)

		for _, name := range keys {
			pattern := s.patterns[name]

			// Determine which line version to use
			targetLine := line
			// Only apply masking for command checks to avoid false positives from commented code.
			// Secrets should still be found even if commented out.
			if name == "Dangerous Command" || name == "Root Deletion" {
				targetLine = maskedLine
			}

			// If targetLine is empty after masking/trimming, skip
			if strings.TrimSpace(targetLine) == "" {
				continue
			}

			matches := pattern.FindAllStringIndex(targetLine, -1)
			for _, match := range matches {
				matchedText := targetLine[match[0]:match[1]]

				findings = append(findings, Finding{
					Type:        name,
					Description: fmt.Sprintf("Found potential %s", name),
					Match:       matchedText,
					Line:        i + 1,
				})
			}
		}

		// Additional heuristic checks (optional) - Preserved
		if strings.Contains(strings.ToLower(line), "password") && strings.Contains(line, "=") {
			// No-op
		}
	}

	return findings, nil
}

// maskComments replaces comments (including inline) with spaces, preserving length.
func maskComments(line string) string {
	b := []byte(line)
	n := len(b)
	inQuote := false
	var quoteChar byte
	escaped := false

	for i := 0; i < n; i++ {
		c := b[i]

		if inQuote {
			if escaped {
				escaped = false
			} else if c == '\\' {
				// Backslash escapes are only active in double quotes (roughly)
				// But in single quotes in bash, everything is literal.
				if quoteChar == '"' {
					escaped = true
				}
			} else if c == quoteChar {
				inQuote = false
			}
		} else {
			if c == '"' || c == '\'' {
				inQuote = true
				quoteChar = c
			} else if c == '#' {
				// Check if this '#' starts a comment
				isComment := false
				if i == 0 {
					isComment = true
				} else {
					// Check preceding character for delimiter
					prev := b[i-1]
					// Bash delimiters: space, tab, newline (not here), ;, &, |, (, ), <, >
					if prev == ' ' || prev == '\t' || strings.ContainsRune(";&|()<>", rune(prev)) {
						isComment = true
					}
				}

				if isComment {
					// Mask the rest of the line with spaces
					for j := i; j < n; j++ {
						b[j] = ' '
					}
					// Since we masked the rest, we can stop
					break
				}
			}
		}
	}
	return string(b)
}
