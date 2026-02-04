package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizer_Sanitize(t *testing.T) {
	s := NewSanitizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Email",
			input:    "Contact me at user@example.com for info.",
			expected: "Contact me at [REDACTED_EMAIL] for info.",
		},
		{
			name:     "IPv4",
			input:    "Server IP is 192.168.1.1 and port 80.",
			expected: "Server IP is [REDACTED_IPV4] and port 80.",
		},
		{
			name:     "Credit Card",
			input:    "Card: 1234-5678-1234-5678.",
			expected: "Card: [REDACTED_CREDIT_CARD].",
		},
		{
			name:     "SSN",
			input:    "My SSN is 123-45-6789.",
			expected: "My SSN is [REDACTED_SSN].",
		},
		{
			name:     "Phone",
			input:    "Call 555-123-4567 now.",
			expected: "Call [REDACTED_PHONE] now.",
		},
		{
			name:     "MAC Address",
			input:    "MAC: 00:1A:2B:3C:4D:5E.",
			expected: "MAC: [REDACTED_MAC].",
		},
		{
			name:     "API Key",
			input:    "api_key = 'abcdef1234567890abcdef1234567890'",
			expected: "api_key = '[REDACTED_API_KEY]'",
		},
		{
			name:     "Multiple",
			input:    "Email: foo@bar.com, IP: 10.0.0.1",
			expected: "Email: [REDACTED_EMAIL], IP: [REDACTED_IPV4]",
		},
		{
			name:     "No PII",
			input:    "Just some text.",
			expected: "Just some text.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.Sanitize(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
