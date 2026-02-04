package security

import (
	"regexp"
	"strings"
)

// Sanitizer provides functionality to redact PII from text.
type Sanitizer struct {
	patterns map[string]*regexp.Regexp
}

// NewSanitizer creates a new Sanitizer with default patterns.
func NewSanitizer() *Sanitizer {
	return &Sanitizer{
		patterns: map[string]*regexp.Regexp{
			"EMAIL":       regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
			"IPV4":        regexp.MustCompile(`\b(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`),
			"CREDIT_CARD": regexp.MustCompile(`\b(?:\d{4}[- ]?){3}\d{4}\b`),
			"SSN":         regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
			// Phone is hard, simple US format for now: (123) 456-7890 or 123-456-7890
			"PHONE": regexp.MustCompile(`\b\(?\d{3}\)?[-. ]?\d{3}[-. ]?\d{4}\b`),
			"MAC":   regexp.MustCompile(`\b([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})\b`),
			// Generic API Key (High entropy string) - reuse heuristic
			"API_KEY": regexp.MustCompile(`(api|access|secret)[_-]?key\s*[:=]\s*['"]([a-zA-Z0-9_\-]{20,})['"]`),
		},
	}
}

// Sanitize replaces PII with [REDACTED_<TYPE>].
func (s *Sanitizer) Sanitize(text string) string {
	sanitized := text
	for name, re := range s.patterns {
		if name == "API_KEY" {
			// Special handling for API keys to preserve the key name but redact the value
			sanitized = re.ReplaceAllStringFunc(sanitized, func(match string) string {
				// match is like "api_key = 'abcdef...'"
				// We want "api_key = '[REDACTED_API_KEY]'"
				submatches := re.FindStringSubmatch(match)
				if len(submatches) >= 3 {
					// Replace the value part
					val := submatches[2]
					return strings.Replace(match, val, "[REDACTED_API_KEY]", 1)
				}
				return match
			})
			continue
		}

		sanitized = re.ReplaceAllString(sanitized, "[REDACTED_"+name+"]")
	}
	return sanitized
}
