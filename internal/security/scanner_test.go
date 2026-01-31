package security

import (
	"testing"
)

func TestRegexScanner_Scan(t *testing.T) {
	scanner := NewRegexScanner()

	tests := []struct {
		name        string
		content     string
		wantFinding string
	}{
		{
			name:        "Safe Content",
			content:     "fmt.Println(\"Hello World\")",
			wantFinding: "",
		},
		{
			name:        "AWS Key",
			content:     "var key = \"AKIAIOSFODNN7EXAMPLE\"",
			wantFinding: "AWS Access Key",
		},
		{
			name:        "GitHub Token",
			content:     "token = \"ghp_123456789012345678901234567890123456\"",
			wantFinding: "GitHub Token",
		},
		{
			name:        "Private Key",
			content:     "-----BEGIN RSA PRIVATE KEY-----\nMIIEpQIBAAKCAQEA...",
			wantFinding: "Private Key",
		},
		{
			name:        "Generic API Key",
			content:     "api_key = \"abc1234567890abc1234567890\"",
			wantFinding: "Generic API Token",
		},
		{
			name:        "Root Deletion",
			content:     "rm -rf /",
			wantFinding: "Root Deletion",
		},
		{
			name:        "Root Deletion Tilde",
			content:     "rm -rf ~",
			wantFinding: "Root Deletion",
		},
		{
			name:        "Root Deletion Multiline",
			content:     "echo start\nrm -rf /\necho end",
			wantFinding: "Root Deletion",
		},
		{
			name:        "Safe Star Deletion",
			content:     "rm -rf *",
			wantFinding: "",
		},
		{
			name:        "Safe Relative Deletion",
			content:     "rm -rf myfolder",
			wantFinding: "",
		},
		{
			name:        "Safe Tmp Deletion",
			content:     "rm -rf /tmp/foo",
			wantFinding: "",
		},
		// False Positive Cases
		{
			name:        "Command inside double quotes",
			content:     `echo "rm -rf /"`,
			wantFinding: "",
		},
		{
			name:        "Command inside double quotes with space",
			content:     `echo "rm -rf / "`,
			wantFinding: "",
		},
		{
			name:        "Command inside single quotes",
			content:     `echo 'rm -rf /'`,
			wantFinding: "",
		},
		{
			name:        "Command inside comment",
			content:     `# rm -rf /`,
			wantFinding: "",
		},
		{
			name:        "Command inside inline comment",
			content:     `echo hello # rm -rf /`,
			wantFinding: "",
		},
		{
			name:        "Secret in comment (Should be detected)",
			content:     `# key = "AKIAIOSFODNN7EXAMPLE"`,
			wantFinding: "AWS Access Key",
		},
		// Regression Test for Backticks
		{
			name:        "Command Substitution Backticks",
			content:     "echo `rm -rf /`",
			wantFinding: "Root Deletion",
		},
		{
			name:        "Safe Config File (suffix)",
			content:     "cat my.config",
			wantFinding: "",
		},
		{
			name:        "Safe Config File (JSON)",
			content:     "cat config.json",
			wantFinding: "",
		},
		{
			name:        "Dangerous Config File (Hidden)",
			content:     "cat .config",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Dangerous Config File (Path)",
			content:     "cat foo/.config",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Dangerous SSH Key (Tilde)",
			content:     "cat ~/.ssh/id_rsa",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Dangerous SSH Key (Relative)",
			content:     "cat .ssh/id_rsa",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Dangerous Etc Passwd",
			content:     "cat /etc/passwd",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Safe Delete Config",
			content:     "rm -rf some.config",
			wantFinding: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := scanner.Scan(tt.content)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}

			if tt.wantFinding == "" {
				if len(findings) > 0 {
					t.Errorf("Expected no findings, got %d: %v", len(findings), findings)
				}
			} else {
				if len(findings) == 0 {
					t.Errorf("Expected finding %q, got none", tt.wantFinding)
				} else {
					found := false
					for _, f := range findings {
						if f.Type == tt.wantFinding {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected finding type %q, got %v", tt.wantFinding, findings)
					}
				}
			}
		})
	}
}

func TestScanner_Determinism(t *testing.T) {
	scanner := NewRegexScanner()
	content := `
rm -rf /
var key = "AKIAIOSFODNN7EXAMPLE"
`

	// Run multiple times to ensure order is consistent
	for i := 0; i < 10; i++ {
		findings, err := scanner.Scan(content)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}

		if len(findings) != 2 {
			t.Fatalf("Expected 2 findings, got %d", len(findings))
		}

		// Findings should be sorted by key (map iteration order is random, but we sort keys)
		// "AWS Access Key" < "Root Deletion"
		if findings[0].Type != "AWS Access Key" {
			t.Errorf("Iteration %d: Expected first finding 'AWS Access Key', got '%s'", i, findings[0].Type)
		}
		if findings[1].Type != "Root Deletion" {
			t.Errorf("Iteration %d: Expected second finding 'Root Deletion', got '%s'", i, findings[1].Type)
		}
	}
}
