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
	reDangerousCmd    = regexp.MustCompile(`(?i)\b(rm|cat|cp|mv|chmod|chown)\b.*?[\s/<>\"'\\](\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow|\.env|\.git-credentials|\.netrc|/proc/self/environ)`)
	reRootDeletion    = regexp.MustCompile(`(?i)\brm\s+-[rRf]+\s+([/~*]+|/)$`)
	rePipeShell       = regexp.MustCompile(`(?i)(curl|wget)\s+.*?\|\s*(bash|sh|zsh|python|perl|php|ruby)`)
	reReverseShell    = regexp.MustCompile(`(?i)nc\s+.*?-e\s+.*`)
	reDockerSocket    = regexp.MustCompile(`(?i)(-v|--volume|--mount)\s+.*docker\.sock`)
	rePrivileged      = regexp.MustCompile(`(?i)--privileged`)
	reNetRecon        = regexp.MustCompile(`(?i)\b(nmap|masscan)\b`)
)

// NewRegexScanner creates a new scanner with default patterns
func NewRegexScanner() *RegexScanner {
	return &RegexScanner{
		patterns: map[string]*regexp.Regexp{
			"AWS Access Key":       reAWSAccessKey,
			"Private Key":          rePrivateKey,
			"Generic API Token":    reGenericAPIToken,
			"Slack Token":          reSlackToken,
			"GitHub Token":         reGitHubToken,
			"Dangerous Command":    reDangerousCmd,
			"Root Deletion":        reRootDeletion,
			"Pipe to Shell":        rePipeShell,
			"Reverse Shell":        reReverseShell,
			"Docker Socket Mount":  reDockerSocket,
			"Privileged Container": rePrivileged,
			"Network Recon":        reNetRecon,
		},
	}
}

// Scan checks the content for security patterns
func (s *RegexScanner) Scan(content string) ([]Finding, error) {
	var findings []Finding
	var newlines []int

	for name, pattern := range s.patterns {
		matches := pattern.FindAllStringIndex(content, -1)
		for _, match := range matches {
			// ⚡ Bolt: Lazily calculate newlines only if a security issue is found
			if newlines == nil {
				// Pre-calculate newline indices for O(log L) line number lookup
				// ⚡ Bolt: Pre-allocate capacity using strings.Count to prevent allocations
				newlines = make([]int, 0, strings.Count(content, "\n"))
				for i := 0; i < len(content); i++ {
					if content[i] == '\n' {
						newlines = append(newlines, i)
					}
				}
			}

			start := match[0]
			// Binary search to find how many newlines are before the start index
			// sort.SearchInts returns the smallest index i such that newlines[i] >= start
			// If newlines[i] >= start, it means the newline is at or after the start.
			// All newlines before index i are strictly less than start, so they contribute to line count.
			// Line number is count of previous newlines + 1.
			lineNumber := sort.SearchInts(newlines, start) + 1

			matchedText := content[match[0]:match[1]]

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
