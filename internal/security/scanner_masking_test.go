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
			name:        "Real Dangerous Command Quoted Path",
			content:     "rm -rf \"/etc/passwd\"",
			shouldBlock: true,
			expectedMatch: "rm -rf \"/etc/passwd\"",
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
			findings, err := scanner.Scan(tt.content)
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

func TestMaskContent_Internal(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		maskStrings bool
		expected    string
	}{
		{
			name:        "Basic Comment (MaskComments)",
			input:       "echo hello # comment",
			maskStrings: false,
			expected:    "echo hello          ",
		},
		{
			name:        "Full Line Comment (MaskComments)",
			input:       "# full line comment",
			maskStrings: false,
			expected:    "                   ",
		},
		{
			name:        "String Preserved (MaskComments)",
			input:       "echo \"# not a comment\"",
			maskStrings: false,
			expected:    "echo \"# not a comment\"",
		},
		{
			name:        "String Preserved Single (MaskComments)",
			input:       "echo '# not a comment'",
			maskStrings: false,
			expected:    "echo '# not a comment'",
		},
		{
			name:        "String Masked (MaskAll)",
			input:       "echo \"# not a comment\"",
			maskStrings: true,
			expected:    "echo \"               \"",
		},
		{
			name:        "String Masked Single (MaskAll)",
			input:       "echo '# not a comment'",
			maskStrings: true,
			expected:    "echo '               '",
		},
		{
			name:        "String and Comment (MaskAll)",
			input:       "echo \"foo\" # comment",
			maskStrings: true,
			expected:    "echo \"   \"          ",
		},
		{
			name:        "String with Escapes (MaskAll)",
			input:       "echo \"foo \\\" bar\" # comment",
			maskStrings: true,
			expected:    "echo \"          \"          ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskContent(tt.input, tt.maskStrings)
			if got != tt.expected {
				t.Errorf("maskContent(%q, %v) = %q; want %q", tt.input, tt.maskStrings, got, tt.expected)
			}
			if len(got) != len(tt.input) {
				t.Errorf("Length mismatch! input: %d, output: %d", len(tt.input), len(got))
			}
		})
	}
}
