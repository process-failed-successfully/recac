package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
)

// MockDockerWithDB wraps MockDockerForExec to simulate agent-bridge signal commands
// by updating the DBStore directly.
type MockDockerWithDB struct {
	MockDockerForExec
	DB      db.Store
	Project string
}

func (m *MockDockerWithDB) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	fullCmd := strings.Join(cmd, " ")

	// Intercept agent-bridge signal commands
	if strings.Contains(fullCmd, "agent-bridge signal") {
		// Parse signal name (assuming it's the last argument for simplicity in this test context)
		// e.g. agent-bridge signal --privileged PROJECT_SIGNED_OFF
		parts := strings.Fields(fullCmd)
		if len(parts) > 0 {
			signalName := parts[len(parts)-1]

			// Set signal in DB
			if m.DB != nil {
				if err := m.DB.SetSignal(m.Project, signalName, "true"); err != nil {
					return "", err
				}
			}
		}
		return "Success: " + fullCmd, nil
	}

	// Intercept agent-bridge feature set commands
	// e.g. agent-bridge feature set req-1 --status done
	if strings.Contains(fullCmd, "agent-bridge feature set") {
		parts := strings.Fields(fullCmd)
		// Minimal parsing logic for mock
		var featureID string
		var status string
		var passes bool

		for i, part := range parts {
			if part == "set" && i+1 < len(parts) {
				featureID = parts[i+1]
			}
			if part == "--status" && i+1 < len(parts) {
				status = parts[i+1]
			}
			if part == "--passes" && i+1 < len(parts) {
				if parts[i+1] == "true" {
					passes = true
				}
			}
		}

		if featureID != "" && m.DB != nil {
			if err := m.DB.UpdateFeatureStatus(m.Project, featureID, status, passes); err != nil {
				return "", err
			}
		}
		return "Success: " + fullCmd, nil
	}

	return m.MockDockerForExec.Exec(ctx, id, cmd)
}

func (m *MockDockerWithDB) ExecAsUser(ctx context.Context, id string, user string, cmd []string) (string, error) {
	return m.Exec(ctx, id, cmd)
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

	// 5. Initialize DB Store (SQLite in temp dir)
	storeConfig := db.StoreConfig{
		Type:             "sqlite",
		ConnectionString: filepath.Join(tmpDir, ".recac.db"),
	}
	dbStore, err := db.NewStore(storeConfig)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}
	// Ensure table init or similar (NewStore usually does it)

	// 6. Initialize Session with DB and Custom Docker Mock
	projectName := "test-project"
	mockDocker := &MockDockerWithDB{
		MockDockerForExec: MockDockerForExec{},
		DB:                dbStore,
		Project:           projectName,
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
		Project:          projectName,
		OwnsDB:           true,
	}

	// 7. Run Loop
	err = s.RunLoop(context.Background())

	// Since all features pass, it should mark COMPLETED and print UI verification msg.
	// We mainly verify it DOESN'T fail or block.
	// ErrNoOp is expected because the MockAgent returns empty responses.
	// OR nil if it exits cleanly via PROJECT_SIGNED_OFF.
	if err != nil && !errors.Is(err, ErrNoOp) {
		t.Errorf("RunLoop failed: %v", err)
	}
}
