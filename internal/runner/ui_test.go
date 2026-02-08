package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"recac/internal/docker"
	"strings"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/security"
	"recac/internal/telemetry"
)

// UIMockDocker is a local mock for UI tests to ensure hermetic execution
type UIMockDocker struct {
	Store db.Store
}

func (m *UIMockDocker) CheckDaemon(ctx context.Context) error { return nil }
func (m *UIMockDocker) RunContainer(ctx context.Context, image, workspace string, extraBinds, env []string, user string) (string, error) {
	return "mock-container-id", nil
}
func (m *UIMockDocker) StopContainer(ctx context.Context, containerID string) error { return nil }

func (m *UIMockDocker) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	// Simulate agent-bridge commands by interacting with the store
	commandStr := strings.Join(cmd, " ")

	// Handle shell wrapping (e.g. /bin/bash -c "...")
	if len(cmd) > 2 && (cmd[0] == "/bin/sh" || cmd[0] == "/bin/bash") && cmd[1] == "-c" {
		commandStr = cmd[2]
	}

	if strings.Contains(commandStr, "agent-bridge signal") {
		parts := strings.Fields(commandStr)
		// Expected: agent-bridge signal <key> <value>
		// Find "signal" index
		sigIdx := -1
		for i, p := range parts {
			if p == "signal" {
				sigIdx = i
				break
			}
		}
		if sigIdx != -1 && sigIdx+2 < len(parts) {
			key := parts[sigIdx+1]
			val := parts[sigIdx+2]
			// Use default project "default" or try to parse from somewhere?
			// The session uses "default" or empty project name in test?
			// Session project is implicit? In test initialization below we rely on NewStore defaulting.
			// Session constructor in test doesn't set project, so it defaults to "unknown" in NewSession,
			// but we initialize struct manually.
			// Let's assume project is empty string or check Session usage.
			// DB operations in Session use s.Project.
			// In test below, s.Project is empty string (zero value).
			// Store uses "default" if empty? No, store uses exact string.
			// Let's check TestSession_RunLoop_UIVerification setup.

			// We need to match the project ID used by Session.
			// We'll try empty string first.
			if m.Store != nil {
				_ = m.Store.SetSignal("", key, val)
			}
			return "Signal set", nil
		}
	}

	if strings.Contains(commandStr, "agent-bridge feature set") {
		// agent-bridge feature set <id> --status <status> --passes <bool>
		// Simplified simulation: just mark it done in store?
		// We don't have easy parsing for args here without a full parser.
		// But for [PRIMES] scenario, the mock agent sends:
		// agent-bridge feature set PRIMES --status done --passes true
		if strings.Contains(commandStr, "PRIMES") && strings.Contains(commandStr, "done") {
			if m.Store != nil {
				_ = m.Store.UpdateFeatureStatus("", "PRIMES", "done", true)
			}
			return "Feature updated", nil
		}
	}

	return "", nil
}

func (m *UIMockDocker) ExecAsUser(ctx context.Context, containerID, user string, cmd []string) (string, error) {
	return m.Exec(ctx, containerID, cmd)
}
func (m *UIMockDocker) PullImage(ctx context.Context, image string) error { return nil }
func (m *UIMockDocker) ImageExists(ctx context.Context, image string) (bool, error) {
	return true, nil
}
func (m *UIMockDocker) ImageBuild(ctx context.Context, options docker.ImageBuildOptions) (string, error) {
	return "mock-image-id", nil
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

	// 5. Initialize DB (Required for signals)
	dbPath := filepath.Join(tmpDir, "test.db")
	dbStore, err := db.NewStore(db.StoreConfig{
		Type:             "sqlite",
		ConnectionString: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to initialize DB: %v", err)
	}
	defer dbStore.Close()

	// 6. Initialize Session
	mockDocker := &UIMockDocker{Store: dbStore}
	mockAgent := agent.NewMockAgent()

	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		CodingAgent:      mockAgent,
		QAAgent:          mockAgent,
		ManagerAgent:     mockAgent,
		Workspace:        tmpDir,
		FeatureContent:   features,
		ManagerFrequency: 5,
		MaxIterations:    50, // Fail fast
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		DBStore:          dbStore, // Critical for signal persistence
		Scanner:          security.NewRegexScanner(),
		SleepFunc:        func(time.Duration) {}, // No sleep
	}

	// 7. Run Loop
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = s.RunLoop(ctx)

	// We verify it completed or hit expected exit condition
	if err != nil && !errors.Is(err, ErrNoOp) && !errors.Is(err, ErrMaxIterations) {
		if err.Error() != "maximum iterations reached" {
			t.Errorf("RunLoop failed: %v", err)
		}
	}
}
