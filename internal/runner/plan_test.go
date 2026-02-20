package runner

import (
	"context"
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

var _ agent.Agent = (*MockPlanAgent)(nil)

type MockPlanAgent struct {
	CapturedPrompt string
	Response       string
}

func (m *MockPlanAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.CapturedPrompt = prompt
	return m.Response, nil
}

func (m *MockPlanAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	m.CapturedPrompt = prompt
	return m.Response, nil
}

func TestSession_RunLoop_PlanOnly(t *testing.T) {
	// Setup Workspace
	tmpDir := t.TempDir()
	specContent := "Build a simple calculator."
	if err := os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock DB
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	// Mock Agent
	mockAgent := &MockPlanAgent{
		Response: "Plan generated.",
	}

	// Initialize Session
	s := &Session{
		Workspace:     tmpDir,
		SpecFile:      "app_spec.txt",
		Agent:         mockAgent,
		DBStore:       store,
		PlanOnly:      true,
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Logger:        telemetry.NewLogger(true, "", false),
		SleepFunc:     func(d time.Duration) {},
		Project:       "test-project",
	}

	ctx := context.Background()
	err := s.RunLoop(ctx)

	if err != nil {
		t.Errorf("RunLoop failed: %v", err)
	}

	// Verify Agent was called with correct prompt
	if mockAgent.CapturedPrompt == "" {
		t.Error("Agent was not called")
	}

	if !strings.Contains(mockAgent.CapturedPrompt, "DO NOT WRITE ANY CODE") {
		t.Error("Agent prompt does not contain Plan-Only instructions")
	}

	if !strings.Contains(mockAgent.CapturedPrompt, specContent) {
		t.Error("Agent prompt does not contain spec")
	}
}
