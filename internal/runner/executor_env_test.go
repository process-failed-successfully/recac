package runner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"recac/internal/notify"
)

// TestEnvLeak_Prevention ensures that environment variables are not leaked during local execution.
func TestEnvLeak_Prevention(t *testing.T) {
	// Set a secret environment variable
	secretKey := "MY_SECRET_KEY"
	secretValue := "supersecret"
	os.Setenv(secretKey, secretValue)
	defer os.Unsetenv(secretKey)

	// Setup Session for local execution
	s := &Session{
		UseLocalAgent: true,
		Project:       "test-project",
		Workspace:     ".", // Use current directory
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Command to print environment variables
	resp := "Checking env.\n```bash\nenv\n```"

	// Execute
	output, err := s.ProcessResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	// Check if secret is leaked
	if strings.Contains(output, secretKey+"="+secretValue) {
		t.Errorf("Environment variable WAS leaked! Secret key '%s' found in output.", secretKey)
	} else {
		t.Logf("Environment variable was NOT leaked as expected.")
	}
}
