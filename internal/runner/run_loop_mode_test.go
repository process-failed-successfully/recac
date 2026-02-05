package runner

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent/prompts"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"strings"
	"testing"
)

func TestSelectPrompt_Modes(t *testing.T) {
	// Setup Session
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)

	s := &Session{
		Workspace:     tmpDir,
		SpecFile:      "app_spec.txt",
		MaxIterations: 5,
		Logger:        telemetry.NewLogger(true, "", false),
	}

	// Test Plan Mode
	s.Mode = "plan"
	prompt, role, _, err := s.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed in plan mode: %v", err)
	}
	if role != prompts.Initializer {
		t.Errorf("Expected Initializer role for plan mode, got %s", role)
	}
	if !strings.Contains(prompt, "YOUR ROLE - INITIALIZER AGENT") {
		t.Errorf("Expected Initializer prompt content, got snippet: %s", prompt[:50])
	}

	// Test Review Mode
	s.Mode = "review"
	prompt, role, _, err = s.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed in review mode: %v", err)
	}
	if role != prompts.Reviewer {
		t.Errorf("Expected Reviewer role for review mode, got %s", role)
	}
	if !strings.Contains(prompt, "YOUR ROLE - REVIEWER AGENT") {
		t.Errorf("Expected Reviewer prompt content, got snippet: %s", prompt[:50])
	}

	// Test Auto Mode (default behavior - should fallback to standard logic)
	s.Mode = "auto"
	// Assuming no features, should default to Initializer
	s.SelectedTaskID = ""
	prompt, role, _, err = s.SelectPrompt()
	if err != nil {
		t.Fatalf("SelectPrompt failed in auto mode: %v", err)
	}
	// With no features, it should be Initializer
	if role != prompts.Initializer {
		t.Errorf("Expected Initializer role for auto mode (no features), got %s", role)
	}
}

func TestRunLoop_Mode_Review_Exit(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	mockAgent := &MockLoopAgent{
		Responses: []string{
			"Review Complete.\n```bash\ncat << 'EOF' > review_report.md\nReport\nEOF\n```",
		},
	}

	s := &Session{
		Workspace:        tmpDir,
		Docker:           &MockLoopDocker{},
		Agent:            mockAgent,
		DBStore:          store,
		MaxIterations:    20, // Should ignore this and exit after 1
		Mode:             "review",
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
	}

	ctx := context.Background()
	err := s.RunLoop(ctx)
	if err != nil {
		t.Errorf("RunLoop failed: %v", err)
	}

	if mockAgent.CallCount != 1 {
		t.Errorf("Expected 1 agent call, got %d", mockAgent.CallCount)
	}
}

// Note: TestRunLoop_Mode_QA requires QAAgent mocking which is harder given it's created inside runQAAgent
// unless we inject it via s.QAAgent. Let's verify if we can inject it.
// Session struct has QAAgent agent.Agent field!
func TestRunLoop_Mode_QA_Exit(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	// Mock QA Agent
	mockQA := &MockLoopAgent{
		Responses: []string{
			"QA Checks.\n```bash\necho QA\nagent-bridge signal QA_PASSED true\n```",
		},
	}

	// Pre-seed DB with QA_PASSED=true so the verification logic passes
	// Actually, runQAAgent calls ProcessResponse which executes commands.
	// But our MockLoopDocker doesn't execute commands locally unless we mock Exec.
	// However, runQAAgent checks DBStore signal.
	// If we pre-seed the signal, runQAAgent should pass.
	// Or we can rely on mockQA executing agent-bridge? No, because we don't have a real agent-bridge or bash execution in unit test.
	// So we must pre-seed the signal in DBStore.
	store.SetSignal("unknown", "QA_PASSED", "true")

	mockDocker := &MockLoopDocker{
		ExecFunc: func(ctx context.Context, containerID string, cmd []string) (string, error) {
			// Join command args
			fullCmd := strings.Join(cmd, " ")
			// Intercept signal command
			if strings.Contains(fullCmd, "agent-bridge signal QA_PASSED true") {
				_ = store.SetSignal("unknown", "QA_PASSED", "true")
			}
			return "Signal Set", nil
		},
	}

	s := &Session{
		Workspace:        tmpDir,
		Docker:           mockDocker,
		QAAgent:          mockQA,
		DBStore:          store,
		Mode:             "qa",
		Notifier:         notify.NewManager(func(string, ...interface{}) {}),
		Logger:           telemetry.NewLogger(true, "", false),
		Project:          "unknown",
	}

	ctx := context.Background()
	err := s.RunLoop(ctx)
	if err != nil {
		t.Errorf("RunLoop failed: %v", err)
	}

	// Verify QA agent was called
	if mockQA.CallCount != 1 {
		t.Errorf("Expected 1 QA agent call, got %d", mockQA.CallCount)
	}
}
