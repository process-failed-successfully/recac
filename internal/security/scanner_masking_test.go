package security

import (
	"testing"
)

func TestScanner_Reproduction_FalsePositives(t *testing.T) {
	scanner := NewRegexScanner()

	tests := []struct {
		name          string
		content       string
		shouldBlock   bool
		expectedMatch string
	}{
		{
			name:        "Commented Dangerous Command",
			content:     "# rm -rf /etc/passwd",
			shouldBlock: false,
		},
		{
			name:        "Inline Commented Dangerous Command",
			content:     "echo hello # rm -rf /etc/passwd",
			shouldBlock: false,
		},
		{
			name:        "Dangerous Command in String (Double Quotes)",
			content:     "echo \"rm -rf /etc/passwd\"",
			shouldBlock: false,
		},
		{
			name:        "Dangerous Command in String (Single Quotes)",
			content:     "echo 'rm -rf /etc/passwd'",
			shouldBlock: false,
		},
		{
			name:        "Real Dangerous Command",
			content:     "rm -rf /etc/passwd",
			shouldBlock: true,
			expectedMatch: "rm -rf /etc/passwd",
		},
		{
			name:        "Real Dangerous Command with trailing comment",
			content:     "rm -rf /etc/passwd # deleting everything",
			shouldBlock: true,
			expectedMatch: "rm -rf /etc/passwd",
		},
		{
			name:        "Root Deletion",
			content:     "rm -rf /",
			shouldBlock: true,
			expectedMatch: "rm -rf /",
		},
		{
			name:        "Root Deletion Commented",
			content:     "# rm -rf /",
			shouldBlock: false,
		},
		{
			name:        "Escaped Command (Backslash)",
			content:     "\\rm -rf /etc/passwd",
			shouldBlock: true,
			expectedMatch: "rm -rf /etc/passwd",
		},
		{
			name:        "Multiline Root Deletion (Bug Fix Check)",
			content:     "echo start\nrm -rf /\necho end",
			shouldBlock: true,
			expectedMatch: "rm -rf /",
		},
		{
			name:        "Secret in Comment (Should still be found)",
			content:     "# Key: AKIAIOSFODNN7EXAMPLE",
			shouldBlock: true, // It is a finding, but maybe not 'blocked' in executor logic depending on type?
			// Executor logic blocks if ANY finding is returned.
			// But here we are testing the scanner.
			expectedMatch: "AKIAIOSFODNN7EXAMPLE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := scanner.Scan("test.sh", tt.content)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}

			found := false
			for _, f := range findings {
				// We care if it found something relevant to the test case
				// For "Secret in Comment", the type is AWS Access Key.
				// For others, it's Dangerous Command or Root Deletion.
				if tt.shouldBlock {
					found = true
					break
				} else {
					// If we expect NO block, any finding is bad IF it is a Dangerous Command.
					// But if it is a Secret, we might expect it?
					// In this test suite, we mostly check for command blocking false positives.
					// But let's be strict: if shouldBlock is false, we expect 0 findings for commands.
					if f.Type == "Dangerous Command" || f.Type == "Root Deletion" {
						t.Errorf("Unexpected finding: %s: %s", f.Type, f.Match)
					}
				}
			}

			if tt.shouldBlock && !found {
				t.Errorf("Expected finding but got none")
			}

			// For the secret case, we want to ensure it WAS found
			if tt.name == "Secret in Comment (Should still be found)" {
				secretFound := false
				for _, f := range findings {
					if f.Type == "AWS Access Key" {
						secretFound = true
					}
				}
				if !secretFound {
					t.Errorf("Expected secret to be found in comment")
				}
			}
		})
	}
}

func TestMaskComments_Internal(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "echo hello # comment",
			expected: "echo hello          ",
		},
		{
			input:    "# full line comment",
			expected: "                   ",
		},
		{
			input:    "echo \"# not a comment\"",
			expected: "echo \"# not a comment\"",
		},
		{
			input:    "echo '# not a comment'",
			expected: "echo '# not a comment'",
		},
		{
			input:    "echo \"foo\" # comment",
			expected: "echo \"foo\"          ",
		},
		{
			input:    "echo \"foo \\\" bar\" # comment",
			expected: "echo \"foo \\\" bar\"          ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := maskShellComments(tt.input)
			if got != tt.expected {
				t.Errorf("maskShellComments(%q) = %q; want %q", tt.input, got, tt.expected)
			}
			if len(got) != len(tt.input) {
				t.Errorf("Length mismatch! input: %d, output: %d", len(tt.input), len(got))
			}
		})
	}
}
