package security

import (
	"strings"
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
	// Create content that triggers multiple rules
	// "rm -rf /" triggers Root Deletion (must be at end of string due to regex anchor)
	// "api_key = ..." triggers Generic API Token
	content := "api_key = '1234567890123456789012345'\nrm -rf /"

	var firstFindings string
	for i := 0; i < 50; i++ {
		findings, err := scanner.Scan(content)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}

		if len(findings) < 2 {
			t.Fatalf("Expected at least 2 findings, got %d", len(findings))
		}

		// Concatenate finding types to check order
		var types []string
		for _, f := range findings {
			types = append(types, f.Type)
		}
		currentFindings := strings.Join(types, ",")

		if firstFindings == "" {
			firstFindings = currentFindings
		} else {
			if firstFindings != currentFindings {
				t.Fatalf("Non-deterministic findings order at iter %d: expected %s, got %s", i, firstFindings, currentFindings)
			}
		}
	}
}
