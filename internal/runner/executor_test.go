package runner

import "testing"

func TestMaskSecrets(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"API Key assignment", "export API_KEY=12345secret", "export API_KEY=[REDACTED]"},
		{"Password in string", "var password = 'supersecret'", "var password = [REDACTED]"},
		{"Token in JSON", `{"token": "abcdef123456"}`, `{"token": [REDACTED]}`},
		{"Secret with quotes", "SECRET=\"my-secret-key\"", "SECRET=[REDACTED]"},
		{"Multiple secrets", "token=123 password=456", "token=[REDACTED] password=[REDACTED]"},
		{"Key assignment", "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE", "AWS_ACCESS_KEY_ID=[REDACTED]"},
		{"API Key assignment (lowercase)", "export api_key=12345secret", "export api_key=[REDACTED]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSecrets(tt.input)
			if result != tt.expected {
				t.Errorf("maskSecrets(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}
