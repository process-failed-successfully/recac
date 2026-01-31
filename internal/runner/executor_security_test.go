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
	var executedCmds []string
	mockDocker := &MockDockerClient{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			fullCmd := strings.Join(cmd, " ")
			// Return empty for blocker check to avoid triggering blocker logic
			if strings.Contains(fullCmd, "recac_blockers.txt") || strings.Contains(fullCmd, "blockers.txt") {
				return "", nil
			}
			executedCmds = append(executedCmds, fullCmd)
			return "mock output", nil
		},
	}

	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      slog.Default(),
		Scanner:     security.NewRegexScanner(), // Use real scanner
	}

	// 1. Dangerous Command
	resp := "I will delete everything.\n```bash\nrm -rf /\n```"
	out, err := s.ProcessResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	// Verify it was blocked
	if !strings.Contains(out, "[BLOCKED] Command 1 blocked by security scanner") {
		t.Errorf("Expected blocked message, got: %s", out)
	}

	if !strings.Contains(out, "Root Deletion") { // Description from scanner
		t.Errorf("Expected description 'Root Deletion', got: %s", out)
	}

	// Verify it was NOT executed
	for _, executed := range executedCmds {
		if strings.Contains(executed, "rm -rf /") {
			t.Errorf("Dangerous command was executed! %s", executed)
		}
	}

	// 2. Safe Command (Sanity Check)
	respSafe := "I will list files.\n```bash\nls -la\n```"
	outSafe, err := s.ProcessResponse(context.Background(), respSafe)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	if strings.Contains(outSafe, "[BLOCKED]") {
		t.Errorf("Safe command was blocked! %s", outSafe)
	}

	// Verify it WAS executed
	found := false
	for _, executed := range executedCmds {
		if strings.Contains(executed, "ls -la") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Safe command was NOT executed")
	}
}

func TestProcessResponse_MockAgentSafe(t *testing.T) {
	var executedCmds []string
	mockDocker := &MockDockerClient{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			fullCmd := strings.Join(cmd, " ")
			// Return empty for blocker check
			if strings.Contains(fullCmd, "recac_blockers.txt") || strings.Contains(fullCmd, "blockers.txt") {
				return "", nil
			}
			executedCmds = append(executedCmds, strings.Join(cmd, " "))
			return "mock output", nil
		},
	}

	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      slog.Default(),
		Scanner:     security.NewRegexScanner(),
	}

	// Mock Agent Script (from internal/agent/mock.go)
	script := `cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.py
git add -f primes.json
git commit -m "Add primes script and output"
`
	resp := "Here is the implementation:\n```bash\n" + script + "\n```"

	out, err := s.ProcessResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	if strings.Contains(out, "[BLOCKED]") {
		t.Errorf("Mock Agent script was blocked! %s", out)
	}

	// Verify execution
	if len(executedCmds) == 0 {
		t.Errorf("Mock Agent script was NOT executed")
	}
}
