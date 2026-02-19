package runner

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"testing"
	"time"
)

func TestSession_RunLoop_PlanOnly(t *testing.T) {
	// Setup workspace
	workspace := t.TempDir()
	specPath := filepath.Join(workspace, "app_spec.txt")
	if err := os.WriteFile(specPath, []byte("Build a simple calculator."), 0644); err != nil {
		t.Fatalf("Failed to create spec file: %v", err)
	}

	// Mock DB
	dbPath := filepath.Join(workspace, ".recac.db")
	store, err := db.NewStore(db.StoreConfig{
		Type:             "sqlite",
		ConnectionString: dbPath,
	})
	if err != nil {
		t.Fatalf("Failed to create DB store: %v", err)
	}
	defer store.Close()

	featureJSON := `{
		"project_name": "Calculator",
		"features": [
			{
				"id": "feat-1",
				"description": "Add functionality",
				"status": "pending",
				"steps": ["Implement add function"],
				"dependencies": { "exclusive_write_paths": [], "read_only_paths": [] }
			}
		]
	}`

	// Agent response should be a command to write this JSON to file
	// Note: The agent response is processed by ProcessResponse, which we are NOT mocking here directly,
	// but we are relying on Session.ProcessResponse calling Docker.Exec.
	// Since we mock Docker.Exec, the actual command string doesn't matter much as long as it triggers something we can intercept,
	// OR we just assume the loop proceeds.
	// BUT, s.loadFeatures() reads from disk.
	// So our mock Docker Exec MUST write the file to disk.

	agentResponse := "I will create the plan.\n```bash\ncat <<EOF > feature_list.json\n" + featureJSON + "\nEOF\n```"

	mockAgent := &MockLoopAgent{Responses: []string{agentResponse}}

	mockDocker := &MockLoopDocker{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			// Simulate writing the file
			// In real execution, ProcessResponse parses the markdown code block and runs it.
			// The command would be "cat <<EOF > feature_list.json..."
			// We just simulate the EFFECT of that command.
			filePath := filepath.Join(workspace, "feature_list.json")
			if err := os.WriteFile(filePath, []byte(featureJSON), 0644); err != nil {
				return "", err
			}
			return "", nil
		},
		CheckDaemonFunc: func(ctx context.Context) error { return nil },
		RunContainerFunc: func(ctx context.Context, imageRef string, workspace string, extraBinds []string, env, cmd []string, user string) (string, error) {
			return "mock-container", nil
		},
		WaitContainerFunc: func(ctx context.Context, containerID string) (int64, error) {
			return 0, nil
		},
		StopContainerFunc: func(ctx context.Context, containerID string) error {
			return nil
		},
	}

	session := &Session{
		Workspace:        workspace,
		Docker:           mockDocker,
		Agent:            mockAgent,
		DBStore:          store,
		MaxIterations:    5,
		ManagerFrequency: 5,
		PlanOnly:         true, // TEST TARGET
		SleepFunc:        func(d time.Duration) {},
		Logger:           telemetry.NewLogger(true, "", false),
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
	}

	// Run Loop
	ctx := context.Background()
	err = session.RunLoop(ctx)
	if err != nil {
		t.Fatalf("RunLoop failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(workspace, "feature_list.json")); os.IsNotExist(err) {
		t.Error("feature_list.json was not created")
	}

	// Verify Agent was called exactly once
	if mockAgent.CallCount != 1 {
		t.Errorf("Agent called %d times, expected 1", mockAgent.CallCount)
	}
}
