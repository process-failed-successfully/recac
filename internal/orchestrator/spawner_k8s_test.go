package orchestrator

import (
	"log/slog"
	"strings"
	"testing"
)

func TestGenerateStartCommand(t *testing.T) {
	spawner := &K8sSpawner{
		Image:  "test-image",
		Logger: slog.Default(),
	}

	item := WorkItem{
		ID:      "TEST-123",
		RepoURL: "https://github.com/org/repo",
	}

	cmd := spawner.generateStartCommand(item)

	expectedChecks := []string{
		`TOKEN=""`,
		`if [ -n "$GITHUB_TOKEN" ]; then`,
		`elif [ -n "$GITHUB_API_KEY" ]; then`,
		`elif [ -n "$RECAC_GITHUB_API_KEY" ]; then`,
		`git config --global url."https://${TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"`,
		`recac-agent --jira "TEST-123"`,
	}

	for _, check := range expectedChecks {
		if !strings.Contains(cmd, check) {
			t.Errorf("Generated command missing expected substring: %s\nCommand:\n%s", check, cmd)
		}
	}
}
