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
	for _, executed := range mockDocker.ExecutedCmds {
		if strings.Contains(executed, "rm -rf /") {
			t.Errorf("Dangerous command was executed! %s", executed)
		}
	}

	// 1.5 Dangerous Command (Wildcard) - Should be ALLOWED now
	respWild := "I will delete all files.\n```bash\nrm -rf *\n```"
	outWild, err := s.ProcessResponse(context.Background(), respWild)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if strings.Contains(outWild, "[BLOCKED]") {
		t.Errorf("Wildcard deletion was blocked! %s", outWild)
	}

	// Verify it WAS executed
	foundWild := false
	for _, executed := range mockDocker.ExecutedCmds {
		if strings.Contains(executed, "rm -rf *") {
			foundWild = true
			break
		}
	}
	if !foundWild {
		t.Errorf("Wildcard deletion command was NOT executed")
	}

	// 1.6 Dangerous Command (Wildcard Root) - Should be BLOCKED
	respWildRoot := "I will delete root files.\n```bash\nrm -rf /*\n```"
	outWildRoot, err := s.ProcessResponse(context.Background(), respWildRoot)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if !strings.Contains(outWildRoot, "[BLOCKED]") {
		t.Errorf("Root wildcard deletion was NOT blocked! %s", outWildRoot)
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
	for _, executed := range mockDocker.ExecutedCmds {
		if strings.Contains(executed, "ls -la") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Safe command was NOT executed")
	}
}
