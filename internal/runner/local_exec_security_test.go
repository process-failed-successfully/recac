package runner

import (
	"context"
	"log/slog"
	"os"
	"recac/internal/notify"
	"recac/internal/security"
	"strings"
	"testing"
)

func TestLocalExecution_EnvLeak(t *testing.T) {
	// Set a secret env var in the test process
	secretKey := "SUPER_SECRET_KEY"
	secretValue := "123456"
	os.Setenv(secretKey, secretValue)
	defer os.Unsetenv(secretKey)

	// Create a temp workspace
	tempDir, err := os.MkdirTemp("", "recac-test-workspace")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup Session with UseLocalAgent = true
	s := &Session{
		UseLocalAgent: true,
		Workspace:     tempDir,
		Project:       "test-project",
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Logger:        slog.Default(),
		Scanner:       security.NewRegexScanner(),
	}

	// Execute a command that prints env
	// We use 'env' command inside a bash block
	resp := "Here is the environment:\n```bash\nenv\n```"
	output, err := s.ProcessResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	// Check if secret leaked into the output
	if strings.Contains(output, secretKey+"="+secretValue) {
		t.Errorf("Security Vulnerability: Host environment variable %s leaked into agent execution!", secretKey)
	} else {
		t.Logf("Verified: %s was correctly filtered from environment output", secretKey)
	}

	// Verify PATH is present (regression test)
	if !strings.Contains(output, "PATH=") {
		t.Errorf("Regression: PATH variable missing from environment!")
	}
}
