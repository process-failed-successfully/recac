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

	// Updated: Use strict path boundaries (non-word chars) to avoid false positives (e.g. my.config.json)
	// while capturing quoted paths (e.g. ".config").
	reDangerousCmd    = regexp.MustCompile(`(?i)\b(rm|cat|cp|mv|chmod|chown)\b.*(?:[^\w]|^)(\.ssh|\.aws|\.config|\.gemini|/?etc/passwd|/?etc/shadow)(?:[^\w]|$)`)

	reRootDeletion    = regexp.MustCompile(`(?im)\brm\s+-[rRf]+\s+(/+\*?|~(/+\*?)?)(?:$|[\s#;\|&])`)
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

	// Mask comments to prevent false positives (e.g. "# rm -rf /")
	// We replace comment lines with spaces to preserve offsets and line numbers.
	var sb strings.Builder
	sb.Grow(len(content))
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			sb.WriteString(strings.Repeat(" ", len(line)))
		} else {
			sb.WriteString(line)
		}
		if i < len(lines)-1 {
			sb.WriteString("\n")
		}
	}
	maskedContent := sb.String()

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

			matchedText := maskedContent[match[0]:match[1]]

			findings = append(findings, Finding{
				Type:        name,
				Description: fmt.Sprintf("Found potential %s", name),
				Match:       matchedText,
				Line:        lineNumber,
			})
		}
	}

	return findings, nil
}
