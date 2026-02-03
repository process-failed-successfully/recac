package runner

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"recac/internal/notify"
)

func TestExecuteCommandBlock_LocalAgent_EnvInjection(t *testing.T) {
	// Create a temporary workspace
	workspace := t.TempDir()

	// Initialize Session with UseLocalAgent = true
	s := &Session{
		Workspace:     workspace,
		Project:       "test-project-env",
		UseLocalAgent: true,
		DBType:        "sqlite",
		DBURL:         "/tmp/test.db",
		Logger:        slog.Default(),
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
	}

	// Command to print environment variables
	cmdScript := "env"

	// Execute
	// ProcessResponse expects markdown code blocks
	output, err := s.ProcessResponse(context.Background(), "```bash\n"+cmdScript+"\n```")

	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	// Check for injected variables
	if !strings.Contains(output, "RECAC_DB_TYPE=sqlite") {
		t.Errorf("Expected RECAC_DB_TYPE=sqlite in output, got:\n%s", output)
	}
	if !strings.Contains(output, "RECAC_DB_URL=/tmp/test.db") {
		t.Errorf("Expected RECAC_DB_URL=/tmp/test.db in output, got:\n%s", output)
	}
    if !strings.Contains(output, "RECAC_PROJECT_ID=test-project-env") {
        t.Errorf("Expected RECAC_PROJECT_ID=test-project-env in output, got:\n%s", output)
    }
}
