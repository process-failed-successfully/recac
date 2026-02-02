package security

import (
	"fmt"
	"regexp"
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
	reDangerousCmd    = regexp.MustCompile(`(?i)\b(rm|cat|cp|mv|chmod|chown)\b.*(\.ssh|\.aws|\.config|\.gemini|/etc/passwd|/etc/shadow)`)
	reRootDeletion    = regexp.MustCompile(`(?i)\brm\s+-[rRf]+\s+([/~*]+|/)$`)
	// Use boundaries \b to prevent false positives (e.g. sync -e matching nc -e)
	// Use non-greedy match that allows backslash-newline for line continuation but stops at other newlines
	rePipeShell       = regexp.MustCompile(`(?i)\b(curl|wget)\b\s+(?:\\\r?\n|.)*?\|\s*(bash|sh|zsh|python|perl|php|ruby|python2|python3)\b`)
	reReverseShell    = regexp.MustCompile(`(?i)\bnc\b\s+.*?-e\s+.*`)
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
	// Mask C-style comments for relevant file types
	if shouldMaskCComments(filename) {
		content = maskCComments(content)
	}

	var findings []Finding
	lines := strings.Split(content, "\n")

	for name, pattern := range s.patterns {
		matches := pattern.FindAllStringIndex(content, -1)
		for _, match := range matches {
			// Find line number
			start := match[0]
			lineNumber := 1
			for i := 0; i < start; i++ {
				if content[i] == '\n' {
					lineNumber++
				}
			}

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

func shouldMaskCComments(filename string) bool {
	exts := []string{".c", ".cpp", ".h", ".hpp", ".go", ".java", ".js", ".ts", ".rs", ".kt", ".scala", ".swift", ".cs"}
	for _, ext := range exts {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}
	return false
}

// maskCComments replaces C-style comments with spaces, preserving line numbers and respecting string literals
func maskCComments(input string) string {
	var output strings.Builder
	runes := []rune(input)
	length := len(runes)

	inLineComment := false
	inBlockComment := false
	inString := false
	stringQuote := rune(0)

	for i := 0; i < length; i++ {
		char := runes[i]

		// Inside Comment
		if inLineComment {
			if char == '\n' {
				inLineComment = false
				output.WriteRune(char)
			} else {
				output.WriteRune(' ')
			}
			continue
		}

		if inBlockComment {
			if char == '*' && i+1 < length && runes[i+1] == '/' {
				inBlockComment = false
				output.WriteRune(' ')
				output.WriteRune(' ')
				i++
			} else if char == '\n' {
				output.WriteRune('\n')
			} else {
				output.WriteRune(' ')
			}
			continue
		}

		// Inside String
		if inString {
			output.WriteRune(char)
			// Handle escape
			if char == '\\' {
				// Skip next char (escaped)
				if i+1 < length {
					output.WriteRune(runes[i+1])
					i++
				}
				continue
			}
			if char == stringQuote {
				inString = false
			}
			continue
		}

		// Check for start of string/char/backtick
		if char == '"' || char == '\'' || char == '`' {
			inString = true
			stringQuote = char
			output.WriteRune(char)
			continue
		}

		// Check for Comment Start
		if i+1 < length && char == '/' && runes[i+1] == '/' {
			inLineComment = true
			output.WriteRune(' ') // Replace /
			output.WriteRune(' ') // Replace /
			i++
			continue
		}

		if i+1 < length && char == '/' && runes[i+1] == '*' {
			inBlockComment = true
			output.WriteRune(' ')
			output.WriteRune(' ')
			i++
			continue
		}

		// Normal Character
		output.WriteRune(char)
	}

	return output.String()
}
