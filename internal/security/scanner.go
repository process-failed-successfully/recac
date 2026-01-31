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

	// Command matching patterns to avoid false positives (e.g. echo "rm -rf /")
	// We require the command to be at the start of a line, or preceded by a separator/grouper, or a known command runner.
	// We use \W+ to handle spaces, quotes, parens etc. after the runner.
	cmdPrefix      = `(?:^|[;&|{(]|\b(?:sudo|xargs|time|nice|nohup|watch|env|start|exec|eval)\W+)\s*`
	reDangerousCmd = regexp.MustCompile(`(?mi)` + cmdPrefix + `\b(rm|cat|cp|mv|chmod|chown)\b.*(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	// reRootDeletion must ensure it matches the specific path and not a prefix (e.g. /tmp), but allow trailing separators/quotes.
	reRootDeletion = regexp.MustCompile(`(?mi)` + cmdPrefix + `\brm\s+-[rRf]+\s+([/~]+|/|/\*)(?:$|[\s;&|)'"])`)
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
	// Mask comments to avoid false positives
	lines := strings.Split(content, "\n")
	maskedLines := make([]string, len(lines))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			// Replace with spaces to preserve byte offsets
			maskedLines[i] = strings.Repeat(" ", len(line))
		} else {
			maskedLines[i] = line
		}
	}
	maskedContent := strings.Join(maskedLines, "\n")

	var findings []Finding

	for name, pattern := range s.patterns {
		matches := pattern.FindAllStringIndex(maskedContent, -1)
		for _, match := range matches {
			// Find line number using original content (though newlines are preserved)
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
