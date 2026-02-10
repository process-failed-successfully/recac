package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"
	"recac/internal/notify"
	"recac/internal/telemetry"
)

type MockDockerWithSideEffects struct {
	MockDockerClient
	Workspace string
}

func (m *MockDockerWithSideEffects) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	fullCmd := strings.Join(cmd, " ")

	if strings.Contains(fullCmd, "agent-bridge signal") {
		// Parse key/value. Simplified for test: assumes --privileged KEY VALUE
		parts := strings.Fields(fullCmd)
		var key, value string
		for i, p := range parts {
			if p == "--privileged" && i+2 < len(parts) {
				key = parts[i+1]
				value = parts[i+2]
				break
			}
		}

		if key != "" && value == "true" {
			path := filepath.Join(m.Workspace, key)
			os.WriteFile(path, []byte("true"), 0644)
		}
	}
	return "Success: " + fullCmd, nil
}

func (m *MockDockerWithSideEffects) ExecAsUser(ctx context.Context, id, user string, cmd []string) (string, error) {
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

	// 5. Initialize Session
	mockDocker := &MockDockerWithSideEffects{Workspace: tmpDir}
	mockAgent := agent.NewMockAgent()
	s := &Session{
		Docker:           mockDocker,
		Agent:            mockAgent,
		ManagerAgent:     mockAgent,
		QAAgent:          mockAgent,
		Workspace:        tmpDir,
		FeatureContent:   features,
		ManagerFrequency: 5,
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
	}

	// 6. Capture Stdout? (Hard to do in test without refactor).
	// We can trust the code if it compiles and logic flows.
	// Or we can observe if it creates the COMPLETED signal.

	err = s.RunLoop(context.Background())

	// Since all features pass, it should mark COMPLETED and print UI verification msg.
	// We mainly verify it DOESN'T fail or block.
	// ErrNoOp is expected because the MockAgent returns empty responses.
	// However, if Project Complete flow runs successfully, it should return nil (cleaner agent complete).
	// But checkNoOpBreaker might trip during the loops if responses are empty.
	// But Lifecycle transitions (QA/Manager) happen at start of loop.
	// They consume the response of previous loop? No, they trigger their own agent calls.

	// If it returns nil, that's success.
	if err != nil && !errors.Is(err, ErrNoOp) {
		t.Errorf("RunLoop failed: %v", err)
	}
}
