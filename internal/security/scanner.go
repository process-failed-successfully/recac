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
	// Improved regexes to avoid matching inside strings/quotes by checking boundaries
	// Added backtick (`) to allowed boundaries to catch command substitution
	// Use non-greedy match .*? to avoid ReDoS and improve performance
	reDangerousCmd = regexp.MustCompile(`(?i)(?:^|[\s;&|()<>` + "`" + `])(rm|cat|cp|mv|chmod|chown)(?:$|[\s;&|()<>` + "`" + `]).*?(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	reRootDeletion = regexp.MustCompile(`(?im)(?:^|[\s;&|()<>` + "`" + `])rm\s+-[rRf]+\s+([/~]+[/*.]*)(?:\s|;|` + "`" + `|$)`)
	// Robust regexes for Pipe/Reverse shell that handle line continuations and boundaries
	rePipeShell    = regexp.MustCompile(`(?i)\b(curl|wget)\b\s+(?:\\\r?\n|.)*?\|\s*(bash|sh|zsh|python|perl|php|ruby)\b`)
	reReverseShell = regexp.MustCompile(`(?i)\bnc\b\s+(?:\\\r?\n|[^;|&])*-e\b`)
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
func (s *RegexScanner) Scan(content string) ([]Finding, error) {
	var findings []Finding
	lines := strings.Split(content, "\n")

	// Pre-compute masked content
	// maskedComments: masks only comments (for standard command checks)
	maskedComments := maskContent(content, false)
	// maskedAll: masks comments AND strings (for pipe/reverse shell checks to avoid matches in strings)
	maskedAll := maskContent(content, true)

	// Sort keys for deterministic output
	keys := make([]string, 0, len(s.patterns))
	for k := range s.patterns {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		pattern := s.patterns[name]
		targetContent := content

		// Select appropriate masked content
		switch name {
		case "Dangerous Command", "Root Deletion":
			targetContent = maskedComments
		case "Pipe to Shell", "Reverse Shell":
			targetContent = maskedAll
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

			// Extract match from original content to show what was found
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
			// Very basic heuristic
		}
		_ = i
	}

	return findings, nil
}

// maskContent replaces comments and optionally string content with spaces.
// If maskStrings is true, content inside single and double quotes is also masked.
func maskContent(content string, maskStrings bool) string {
	bytes := []byte(content)
	n := len(bytes)
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < n; i++ {
		b := bytes[i]

		if escaped {
			escaped = false
			if maskStrings && (inSingleQuote || inDoubleQuote) {
				bytes[i] = ' '
			}
			continue
		}

		if b == '\\' {
			if inSingleQuote {
				// Backslash is literal in single quotes
				if maskStrings {
					bytes[i] = ' '
				}
			} else {
				escaped = true
				if maskStrings && inDoubleQuote {
					bytes[i] = ' '
				}
			}
			continue
		}

		if inSingleQuote {
			if b == '\'' {
				inSingleQuote = false
			} else if maskStrings {
				bytes[i] = ' '
			}
			continue
		}

		if inDoubleQuote {
			if b == '"' {
				inDoubleQuote = false
			} else if maskStrings {
				bytes[i] = ' '
			}
			continue
		}

		// Not in quotes
		if b == '\'' {
			inSingleQuote = true
			continue
		}
		if b == '"' {
			inDoubleQuote = true
			continue
		}

		if b == '#' {
			// Check if it's a valid comment start
			isComment := false
			if i == 0 {
				isComment = true
			} else {
				prev := bytes[i-1]
				// Bash control operators or whitespace
				if prev == ' ' || prev == '\t' || prev == '\n' ||
					prev == ';' || prev == '|' || prev == '&' ||
					prev == '(' || prev == ')' || prev == '<' || prev == '>' {
					isComment = true
				}
			}

			if isComment {
				// Mask until newline
				for j := i; j < n; j++ {
					if bytes[j] == '\n' {
						i = j - 1
						break
					}
					bytes[j] = ' '
					if j == n-1 {
						i = j
					}
				}
			}
		}
	}
	return string(bytes)
}
