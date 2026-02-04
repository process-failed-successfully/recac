package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	// 3. Setup: DBStore (SQLite)
	dbPath := filepath.Join(tmpDir, "test.db")
	dbStore, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	if err != nil {
		t.Fatalf("Failed to create DBStore: %v", err)
	}
	defer dbStore.Close()

	// 4. Setup: Pre-populate Features in DB (to simulate existing features)
	features := []db.Feature{
		{ID: "feat-1", Description: "Feature 1", Status: "pending", Passes: false},
	}
	fl := db.FeatureList{ProjectName: "ui-test", Features: features}
	data, _ := json.Marshal(fl)
	dbStore.SaveFeatures("ui-test", string(data))

	// 5. Initialize Session with MockDockerClient
	mockDocker := &MockDockerClient{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			fullCmd := strings.Join(cmd, " ")

			// IGNORE BLOCKER CHECKS (Critical for avoiding infinite loops)
			if strings.Contains(fullCmd, "recac_blockers.txt") || strings.Contains(fullCmd, "blockers.txt") {
				return "", nil
			}

			// Detect signal setting commands and update DB
			if strings.Contains(fullCmd, "agent-bridge signal") {
				parts := strings.Fields(fullCmd)
				// Format: agent-bridge signal <key> <value>
				// Search for "signal"
				for i, part := range parts {
					if part == "signal" && i+2 < len(parts) {
						signalName := parts[i+1]
						signalValue := parts[i+2]

						// Skip "set" if present (legacy support, though not used by MockAgent anymore)
						if signalName == "set" && i+3 < len(parts) {
							signalName = parts[i+2]
							signalValue = parts[i+3]
						}

						dbStore.SetSignal("ui-test", signalName, signalValue)
						return "Signal set: " + signalName + "=" + signalValue, nil
					}
				}
			}

			// Detect feature setting commands and update DB
			updated := false
			if strings.Contains(fullCmd, "agent-bridge feature set") {
				// Handle piped xargs case specifically (MockAgent output)
				if strings.Contains(fullCmd, "xargs") {
					fs, _ := dbStore.GetFeatures("ui-test")
					var list db.FeatureList
					json.Unmarshal([]byte(fs), &list)
					for _, f := range list.Features {
						dbStore.UpdateFeatureStatus("ui-test", f.ID, "done", true)
					}
					updated = true
				} else {
					// Handle direct feature set
					parts := strings.Fields(fullCmd)
					var id string
					for i, part := range parts {
						if part == "set" && i+1 < len(parts) {
							id = parts[i+1]
							dbStore.UpdateFeatureStatus("ui-test", id, "done", true)
							updated = true
							break
						}
					}
				}
			}

			// Auto-Complete Check (Simulate agent-bridge logic)
			if updated {
				fs, _ := dbStore.GetFeatures("ui-test")
				var list db.FeatureList
				json.Unmarshal([]byte(fs), &list)
				allDone := true
				for _, f := range list.Features {
					if f.Status != "done" || !f.Passes {
						allDone = false
						break
					}
				}
				if allDone {
					dbStore.SetSignal("ui-test", "COMPLETED", "true")
				}
			}

			return "Success: " + fullCmd, nil
		},
	}

	mockAgent := agent.NewMockAgent()
	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		QAAgent:          mockAgent, // Inject mock for QA
		ManagerAgent:     mockAgent, // Inject mock for Manager
		Workspace:        tmpDir,
		Project:          "ui-test",
		DBStore:          dbStore,
		ManagerFrequency: 5,
		MaxIterations:    20, // Limit iterations to prevent infinite loops (timeout protection)
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		SleepFunc:        func(d time.Duration) {}, // Skip sleep for tests
	}

	// 6. Run Loop
	err = s.RunLoop(context.Background())

	if err != nil {
		t.Errorf("RunLoop failed: %v", err)
	}
}
