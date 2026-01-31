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

	// 3. Allowed Wildcard Deletion (Regression Test for #1062)
	// "rm -rf *" should be allowed (current directory), but "rm -rf /" or "rm -rf /*" blocked.
	respWildcard := "I will delete current directory content.\n```bash\nrm -rf *\n```"
	outWildcard, err := s.ProcessResponse(context.Background(), respWildcard)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	if strings.Contains(outWildcard, "[BLOCKED]") {
		t.Errorf("Wildcard deletion (rm -rf *) was blocked! Output: %s", outWildcard)
	}

	// Verify it WAS executed
	foundWildcard := false
	for _, executed := range mockDocker.ExecutedCmds {
		if strings.Contains(executed, "rm -rf *") {
			foundWildcard = true
			break
		}
	}
	if !foundWildcard {
		t.Errorf("Wildcard deletion (rm -rf *) was NOT executed")
	}

	// 4. Blocked Root Wildcard Deletion (Defense against rm -rf /*)
	respRootWildcard := "I will delete root content.\n```bash\nrm -rf /*\n```"
	outRootWildcard, err := s.ProcessResponse(context.Background(), respRootWildcard)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	if !strings.Contains(outRootWildcard, "[BLOCKED]") {
		t.Errorf("Root wildcard deletion (rm -rf /*) was NOT blocked! Output: %s", outRootWildcard)
	}
}
