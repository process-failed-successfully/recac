package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"
	"recac/internal/db"
	"recac/internal/notify"
)

// MockDockerForGuardrail is a simple mock that returns success for everything
type MockDockerForGuardrail struct {
	DockerClient
}

func (m *MockDockerForGuardrail) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	// Simulate "test -f ..." failing so we don't trip legacy blocker checks
	for _, c := range cmd {
		if (len(c) > 0 && c[0] == 't' && c[1] == 'e' && c[2] == 's' && c[3] == 't') ||
			(len(c) > 5 && c[0:4] == "test") ||
			(len(c) > 10 && c[0:3] == "cat" && (c[4:] == "recac_blockers.txt" || c[4:] == "blockers.txt")) {
			return "", fmt.Errorf("file not found")
		}
	}
	// Simplified check: usually the cmd passed to Exec is ["/bin/sh", "-c", "test -f ..."]
	if len(cmd) > 2 && (strings.Contains(cmd[2], "recac_blockers.txt") || strings.Contains(cmd[2], "blockers.txt")) {
		return "", fmt.Errorf("file not found")
	}

	return "ok", nil
}

func TestSession_Guardrail_PrematureSignoff(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Setup Features: One FAILING
	features := `{"features":[{"id":"1","description":"feat","passes":false}]}`
	os.WriteFile(filepath.Join(tmpDir, "feature_list.json"), []byte(features), 0644)
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	// 2. Setup Signals: PROJECT_SIGNED_OFF exists (prematurely)
	// 2. Setup Signals: PROJECT_SIGNED_OFF exists (prematurely)
	// hasSignal looks for a file named "PROJECT_SIGNED_OFF" or DB entry.
	// Since we mock DB but didn't populate it, let's use the file fallback to trigger migration/detection.
	// Actually, now that we have DBStore, hasSignal checks it first.
	// But let's write the file so hasSignal finds it (and potentially migrates it, making it true in DB).
	os.WriteFile(filepath.Join(tmpDir, "PROJECT_SIGNED_OFF"), []byte("true"), 0644)

	mockDocker := &MockDockerForGuardrail{}
	mockAgent := agent.NewMockAgent()

	// Init DB
	dbPath := filepath.Join(tmpDir, "recac.db")
	dbStore, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		Workspace:        tmpDir,
		MaxIterations:    2,
		ManagerFrequency: 5,
		DBStore:          dbStore,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
	}

	// 3. Run Loop
	// We expect the loop to run, see the bad signal, clear it, warn, and CONTINUE.
	// Since we set MaxIterations=2, it should eventually stop by iteration limit.
	// If the guardrail works, it will NOT return "nil" immediately at the start of iteration 1 because of the break,
	// but instead run through iteration 1.

	// However, since we mock everything, it will just loop.
	// We can check if the signal file is gone after the run.

	_ = s.RunLoop(context.Background())

	// 4. Verify Signal is GONE
	if s.hasSignal("PROJECT_SIGNED_OFF") {
		t.Error("Guardrail failed: PROJECT_SIGNED_OFF signal should have been cleared because features are failing.")
	}

	// 4. Verify Signal is GONE
	if s.hasSignal("PROJECT_SIGNED_OFF") {
		t.Error("Guardrail failed: PROJECT_SIGNED_OFF signal should have been cleared because features are failing.")
	}

	// Double check DB
	val, _ := s.DBStore.GetSignal("PROJECT_SIGNED_OFF")
	if val != "" {
		t.Errorf("Guardrail failed: DB still has signal: %s", val)
	}
}
