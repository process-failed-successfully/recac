package security

import (
	"testing"
)

func TestRegexScanner_Scan(t *testing.T) {
	scanner := NewRegexScanner()

	tests := []struct {
		name      string
		content   string
		findings  int // Expected number of findings
		blockedBy string // Expected blocker type (if any)
	}{
		// Safe Cases
		{
			name:     "Safe Content",
			content:  "echo 'Hello World'",
			findings: 0,
		},
		{
			name:     "Allow rm -rf * (current dir)",
			content:  "rm -rf *",
			findings: 0,
		},
		{
			name:     "Allow rm my.config.json",
			content:  "rm my.config.json",
			findings: 0,
		},
		{
			name:     "Allow rm foo.ssh",
			content:  "rm foo.ssh",
			findings: 0,
		},
		{
			name:     "Allow rm with quoted config in name",
			content:  "rm 'my.config'",
			findings: 0,
		},
		// False Positives Fixed
		{
			name:     "Echo dangerous command (single quotes)",
			content:  "echo 'rm -rf /'",
			findings: 0,
		},
		{
			name:     "Echo dangerous path (single quotes)",
			content:  "echo 'cat .ssh/id_rsa'",
			findings: 0,
		},
		{
			name:     "Inline comment",
			content:  "ls -la # rm -rf /",
			findings: 0,
		},
		{
			name:     "Commented out dangerous command",
			content:  "# rm -rf /",
			findings: 0,
		},
		{
			name:     "Escaped double quote in string",
			content:  "echo \"This is NOT a comment # and this is part of string \\\" # still string\"",
			findings: 0,
		},

		// Dangerous Cases
		{
			name:      "AWS Key",
			content:   "AKIAIOSFODNN7EXAMPLE",
			findings:  1,
			blockedBy: "AWS Access Key",
		},
		{
			name:      "GitHub Token",
			content:   "ghp_123456789012345678901234567890123456",
			findings:  1,
			blockedBy: "GitHub Token",
		},
		{
			name:      "Private Key",
			content:   "-----BEGIN RSA PRIVATE KEY-----",
			findings:  1,
			blockedBy: "Private Key",
		},
		{
			name:      "Generic API Key",
			content:   "api_key = '12345678901234567890'",
			findings:  1,
			blockedBy: "Generic API Token",
		},
		{
			name:      "Secret inside double-quoted string with escaped quote",
			// Checks that \" doesn't end the string, so # is part of string (preserved), so secret is found.
			content:   "val = \"text \\\" # api_key = '12345678901234567890'\"",
			findings:  1,
			blockedBy: "Generic API Token",
		},
		{
			name:      "Block rm -rf /",
			content:   "rm -rf /",
			findings:  1,
			blockedBy: "Root Deletion",
		},
		{
			name:      "Block rm -rf ~",
			content:   "rm -rf ~",
			findings:  1,
			blockedBy: "Root Deletion",
		},
		{
			name:      "Block rm -rf /*",
			content:   "rm -rf /*",
			findings:  1,
			blockedBy: "Root Deletion",
		},
		{
			name:      "Block rm .config",
			content:   "rm .config",
			findings:  1,
			blockedBy: "Dangerous Command",
		},
		{
			name:      "Block rm ~/.aws/credentials",
			content:   "rm ~/.aws/credentials",
			findings:  1,
			blockedBy: "Dangerous Command",
		},
		{
			name:      "Block multiline rm -rf /",
			content:   "echo start\nrm -rf /\necho end",
			findings:  1,
			blockedBy: "Root Deletion",
		},
		{
			name:      "Block rm -rf / with trailing whitespace",
			content:   "rm -rf /   ",
			findings:  1,
			blockedBy: "Root Deletion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := scanner.Scan(tt.content)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}

			if len(findings) != tt.findings {
				t.Errorf("Expected %d findings, got %d: %v", tt.findings, len(findings), findings)
			}

			if tt.findings > 0 && tt.blockedBy != "" {
				found := false
				for _, f := range findings {
					if f.Type == tt.blockedBy {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected findings to contain type '%s', got: %v", tt.blockedBy, findings)
				}
			}
		})
	}
}
