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

func TestSession_RunLoop_UIVerification(t *testing.T) {
	// 1. Create a temp directory
	tmpDir, err := os.MkdirTemp("", "ui_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Setup: app_spec.txt (required)
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	// 3. Initialize DB Store (Needed for signals)
	dbPath := filepath.Join(tmpDir, "test.db")
	dbStore, err := db.NewStore(db.StoreConfig{
		Type:             "sqlite",
		ConnectionString: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	// 4. Setup: features in DB
	featuresJSON := `{"features":[{"id":"req-the-list-of-primes-in-primes-j","description":"The list of primes in primes.json contains exactly 1229 primes","status":"pending","passes":false}]}`
	dbStore.SaveFeatures("unknown", featuresJSON)

	// 5. Setup: ui_verification.json (Should be detected)
	os.WriteFile(filepath.Join(tmpDir, "ui_verification.json"), []byte("Verify Button Color"), 0644)

	// 6. Initialize Session with Smart Mock Docker
	mockDocker := &MockDockerClient{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			commandStr := strings.Join(cmd, " ")

			// Detect agent-bridge update call
			if strings.Contains(commandStr, "agent-bridge feature set") && strings.Contains(commandStr, "--status done") {
				// Manually update the DB to simulate bridge action
				// Parse ID is hard, but we know the ID from setup
				updatedJSON := `{"features":[{"id":"req-the-list-of-primes-in-primes-j","description":"The list of primes in primes.json contains exactly 1229 primes","status":"done","passes":true}]}`
				dbStore.SaveFeatures("unknown", updatedJSON)
				return "Success: updated feature", nil
			}

			// Detect agent-bridge signal call (QA/Manager)
			if strings.Contains(commandStr, "agent-bridge signal") {
				parts := strings.Fields(commandStr)
				for i, part := range parts {
					if part == "signal" && i+1 < len(parts) {
						key := parts[i+1]
						value := "true"
						if i+2 < len(parts) {
							value = parts[i+2]
						}
						dbStore.SetSignal("unknown", key, value)
						return "Success: signal set", nil
					}
				}
			}

			// Handle blocker checks (return empty to simulate no blockers)
			if strings.Contains(commandStr, "recac_blockers.txt") || strings.Contains(commandStr, "blockers.txt") {
				return "", nil
			}

			return "Success: " + commandStr, nil
		},
	}

	mockAgent := agent.NewMockAgent()
	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		QAAgent:          mockAgent, // Inject mock for QA phase
		ManagerAgent:     mockAgent, // Inject mock for Manager phase
		Workspace:        tmpDir,
		Project:          "unknown", // Must match DB record
		DBStore:          dbStore,   // Pass the DB Store
		ManagerFrequency: 5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		MaxIterations:    20,
	}

	// 7. Run Loop
	err = s.RunLoop(context.Background())

	// Since feature is updated to done by the mock exec hook, it should eventually pass and mark COMPLETED.
	if err != nil && !errors.Is(err, ErrNoOp) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("RunLoop failed: %v", err)
	}
}
