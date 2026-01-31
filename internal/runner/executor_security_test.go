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

	// 3. Allowed: Star Deletion (rm -rf *)
	respAllowed1 := "I will clean up.\n```bash\nrm -rf *\n```"
	outAllowed1, err := s.ProcessResponse(context.Background(), respAllowed1)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if strings.Contains(outAllowed1, "[BLOCKED]") {
		t.Errorf("Star Deletion should be allowed, but was blocked! %s", outAllowed1)
	}

	// 4. Allowed: Benign Config (cat my.config)
	respAllowed2 := "I will read config.\n```bash\ncat my.config\n```"
	outAllowed2, err := s.ProcessResponse(context.Background(), respAllowed2)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if strings.Contains(outAllowed2, "[BLOCKED]") {
		t.Errorf("Benign Config should be allowed, but was blocked! %s", outAllowed2)
	}
}
