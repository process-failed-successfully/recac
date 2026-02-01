package runner

import (
	"context"
	"log/slog"
	"recac/internal/notify"
	"recac/internal/security"
	"strings"
	"testing"
)

func TestProcessResponse_Security(t *testing.T) {
	mockDocker := &MockDockerForExec{}
	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      slog.Default(),
		Scanner:     security.NewRegexScanner(), // Use real scanner
	}

	testCases := []struct {
		name         string
		input        string
		shouldBlock  bool
		expectedDesc string
		cmdToVerify  string // Command substring to verify execution/blocking
	}{
		{
			name:         "Root Deletion",
			input:        "I will delete everything.\n```bash\nrm -rf /\n```",
			shouldBlock:  true,
			expectedDesc: "Root Deletion",
			cmdToVerify:  "rm -rf /",
		},
		{
			name:         "Root Wildcard Deletion",
			input:        "I will delete everything.\n```bash\nrm -rf /*\n```",
			shouldBlock:  true,
			expectedDesc: "Root Deletion",
			cmdToVerify:  "rm -rf /*",
		},
		{
			name:         "Home Deletion",
			input:        "I will delete home.\n```bash\nrm -rf ~\n```",
			shouldBlock:  true,
			expectedDesc: "Root Deletion",
			cmdToVerify:  "rm -rf ~",
		},
		{
			name:        "Current Dir Deletion (Allowed)",
			input:       "I will delete current dir files.\n```bash\nrm -rf *\n```",
			shouldBlock: false,
			cmdToVerify: "rm -rf *",
		},
		{
			name:        "Safe Command",
			input:       "I will list files.\n```bash\nls -la\n```",
			shouldBlock: false,
			cmdToVerify: "ls -la",
		},
		{
			name:        "False Positive: Echo Root Deletion",
			input:       "I will explain.\n```bash\necho \"rm -rf /\"\n```",
			shouldBlock: false,
			cmdToVerify: "echo \"rm -rf /\"",
		},
		{
			name:        "False Positive: Echo Root Deletion No Quotes",
			input:       "I will explain.\n```bash\necho rm -rf /\n```",
			shouldBlock: false,
			cmdToVerify: "echo rm -rf /",
		},
		{
			name:         "Sudo Root Deletion",
			input:        "I will use sudo.\n```bash\nsudo rm -rf /\n```",
			shouldBlock:  true,
			expectedDesc: "Root Deletion",
			cmdToVerify:  "sudo rm -rf /",
		},
		{
			name:         "Dangerous File Access",
			input:        "I will read ssh key.\n```bash\ncat ~/.ssh/id_rsa\n```",
			shouldBlock:  true,
			expectedDesc: "Dangerous Command",
			cmdToVerify:  "cat ~/.ssh/id_rsa",
		},
		{
			name:        "False Positive: Echo Dangerous File Access",
			input:       "I will explain ssh.\n```bash\necho cat ~/.ssh/id_rsa\n```",
			shouldBlock: false,
			cmdToVerify: "echo cat ~/.ssh/id_rsa",
		},
		{
			name:        "Complex False Positive",
			input:       "I will print commands.\n```bash\nprintf 'rm -rf /'\n```",
			shouldBlock: false, // printf is not in the list of dangerous command runners
			cmdToVerify: "printf 'rm -rf /'",
		},
		{
			name:         "Eval Root Deletion",
			input:        "I will use eval.\n```bash\neval \"rm -rf /\"\n```",
			shouldBlock:  true,
			expectedDesc: "Root Deletion",
			cmdToVerify:  "eval \"rm -rf /\"",
		},
		{
			name:         "Exec Root Deletion",
			input:        "I will use exec.\n```bash\nexec rm -rf /\n```",
			shouldBlock:  true,
			expectedDesc: "Root Deletion",
			cmdToVerify:  "exec rm -rf /",
		},
		{
			name:        "Mock Agent Git Config (Allowed)",
			input:       "Configure git.\n```bash\ngit config user.email \"mock@example.com\"\n```",
			shouldBlock: false,
			cmdToVerify: "git config user.email \"mock@example.com\"",
		},
		{
			name:        "Agent Bridge Import (Allowed)",
			input:       "Import features.\n```bash\nagent-bridge import --file feature_list.json\n```",
			shouldBlock: false,
			cmdToVerify: "agent-bridge import",
		},
		{
			name:        "Agent Bridge Signal Set (Allowed)",
			input:       "Set signal.\n```bash\nagent-bridge signal set QA_PASSED true\n```",
			shouldBlock: false,
			cmdToVerify: "agent-bridge signal set",
		},
		{
			name:        "Agent Bridge Update (Allowed)",
			input:       "Update status.\n```bash\nagent-bridge update --id req-primes --status done --passes true\n```",
			shouldBlock: false,
			cmdToVerify: "agent-bridge update",
		},
		{
			name:        "Python Script Creation (Allowed)",
			input:       "Create script.\n```bash\ncat << 'EOF' > primes.py\nimport json\nEOF\n```",
			shouldBlock: false,
			cmdToVerify: "cat << 'EOF' > primes.py",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear executed cmds
			mockDocker.ExecutedCmds = nil

			out, err := s.ProcessResponse(context.Background(), tc.input)
			if err != nil {
				t.Fatalf("ProcessResponse failed: %v", err)
			}

			if tc.shouldBlock {
				if !strings.Contains(out, "[BLOCKED]") {
					t.Errorf("Expected blocked message for '%s', got: %s", tc.name, out)
				}
				if tc.expectedDesc != "" && !strings.Contains(out, tc.expectedDesc) {
					t.Errorf("Expected description '%s', got: %s", tc.expectedDesc, out)
				}
				// Verify NOT executed
				for _, executed := range mockDocker.ExecutedCmds {
					// Ignore blocker checks
					if strings.Contains(executed, "recac_blockers.txt") || strings.Contains(executed, "test -f blockers.txt") {
						continue
					}
					// Check if the specific command was executed
					if strings.Contains(executed, tc.cmdToVerify) {
						t.Errorf("Blocked command '%s' was executed! Executed: %s", tc.cmdToVerify, executed)
					}
				}
			} else {
				if strings.Contains(out, "[BLOCKED]") {
					t.Errorf("Safe command '%s' was blocked! Output: %s", tc.name, out)
				}
				// Verify executed (if it contains bash block)
				if strings.Contains(tc.input, "```bash") {
					found := false
					for _, executed := range mockDocker.ExecutedCmds {
						if strings.Contains(executed, tc.cmdToVerify) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Safe command '%s' was NOT executed", tc.cmdToVerify)
					}
				}
			}
		})
	}
}
