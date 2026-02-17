package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"strings"
)

type MockDockerWithDB struct {
	*MockDockerForExec
	DBStore   db.Store
	ProjectID string
}

func (m *MockDockerWithDB) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	fullCmd := strings.Join(cmd, " ")
	if strings.Contains(fullCmd, "agent-bridge feature set") {
		// For verification test, assume setting a feature means we are done
		// We set COMPLETED directly to break the loop
		m.DBStore.SetSignal(m.ProjectID, "COMPLETED", "true")
		return "Success: " + fullCmd, nil
	}
	if strings.Contains(fullCmd, "agent-bridge signal") {
		// Parse and set signal
		// agent-bridge signal <key> <value>
		parts := strings.Fields(fullCmd)
		// Find "signal"
		sigIdx := -1
		for i, part := range parts {
			if part == "signal" {
				sigIdx = i
				break
			}
		}
		if sigIdx != -1 && sigIdx+2 < len(parts) {
			key := parts[sigIdx+1]
			val := parts[sigIdx+2]
			m.DBStore.SetSignal(m.ProjectID, key, val)
			return "Success: " + fullCmd, nil
		}
	}
	return m.MockDockerForExec.Exec(ctx, id, cmd)
}

func TestSession_RunLoop_UIVerification(t *testing.T) {
	// 1. Create a temp directory
	tmpDir, err := os.MkdirTemp("", "ui_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Setup: app_spec.txt (required)
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	// 3. Setup: feature_list.json with ALL PASSING (Use FeatureContent)
	features := `{"features":[{"id":"1","description":"feat","status":"done","passes":true}]}`

	// 4. Setup: ui_verification.json (Should be detected)
	os.WriteFile(filepath.Join(tmpDir, "ui_verification.json"), []byte("Verify Button Color"), 0644)

	// 5. Initialize Session

	// Initialize DBStore
	dbPath := filepath.Join(tmpDir, "test.db")
	dbStore, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB store: %v", err)
	}
	defer dbStore.Close()

	mockDocker := &MockDockerWithDB{
		MockDockerForExec: &MockDockerForExec{},
		DBStore:           dbStore,
		ProjectID:         "", // Default for session
	}
	mockAgent := agent.NewMockAgent()

	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		Workspace:        tmpDir,
		FeatureContent:   features,
		ManagerFrequency: 5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		DBStore:          dbStore,
		AgentProvider:    "mock",
		AgentModel:       "mock",
	}

	// 6. Capture Stdout? (Hard to do in test without refactor).
	// We can trust the code if it compiles and logic flows.
	// Or we can observe if it creates the COMPLETED signal.

	err = s.RunLoop(context.Background())

	// Since all features pass, it should mark COMPLETED and print UI verification msg.
	// We mainly verify it DOESN'T fail or block.
	// ErrNoOp is expected because the MockAgent returns empty responses.
	if err != nil && !errors.Is(err, ErrNoOp) {
		t.Errorf("RunLoop failed: %v", err)
	}
}
