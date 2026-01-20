package runner

import (
	"context"
	"log/slog"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"strings"
	"testing"
)

// MockAgentForQA simulates the agent interaction for QA
type MockAgentForQA struct {
	Response  string
	Workspace string
	Store     db.Store
	Project   string
}

func (m *MockAgentForQA) Send(ctx context.Context, prompt string) (string, error) {
	// Simulate agent action: set signal in DB directly
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		if m.Response == "PASS" {
			m.Store.SetSignal(m.Project, "QA_PASSED", "true")
		} else {
			m.Store.SetSignal(m.Project, "QA_PASSED", "false")
		}
		return "I have completed the QA checks.", nil
	}
	return "Unknown prompt", nil
}

func (m *MockAgentForQA) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, nil
}

func (m *MockAgentForQA) SetResponse(response string) {
	m.Response = response
}

func TestRunQAAgent_Pass(t *testing.T) {
	// 1. Setup Workspace
	workspace := t.TempDir()

	// 2. Setup DB
	dbPath := filepath.Join(workspace, ".recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 3. Setup Mock Agent
	mockAgent := &MockAgentForQA{
		Response: "PASS",
		Store:    store,
		Project:  "test-project",
	}

	// 4. Create Session
	session := &Session{
		Agent:     mockAgent,
		QAAgent:   mockAgent,
		Workspace: workspace,
		DBStore:   store,
		// Docker client not needed for this test
		Docker:   nil,
		Project:  "test-project",
		Notifier: notify.NewManager(func(string, ...interface{}) {}),
		Logger:   telemetry.NewLogger(true, "", false),
	}

	// 5. Run QA Agent
	err = session.runQAAgent(context.Background())
	if err != nil {
		t.Errorf("runQAAgent failed: %v", err)
	}

	// 6. Verify Signal
	val, err := store.GetSignal("test-project", "QA_PASSED")
	if err != nil {
		t.Errorf("Failed to get signal: %v", err)
	}
	if val != "true" {
		t.Errorf("Expected QA_PASSED signal 'true', got '%s'", val)
	}
}

func TestRunQAAgent_Fail(t *testing.T) {
	// 1. Setup Workspace
	workspace := t.TempDir()

	// 2. Setup DB
	dbPath := filepath.Join(workspace, ".recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 3. Setup Mock Agent
	mockAgent := &MockAgentForQA{
		Response: "FAIL",
		Store:    store,
		Project:  "test-project",
	}

	// 4. Create Session
	projectName := "test-project"
	s := &Session{
		Workspace: workspace,
		Project:   projectName,
		DBStore:   store,
		Logger:    slog.Default(),
		Notifier:  notify.NewManager(func(string, ...interface{}) {}),
		Agent:     mockAgent,
		QAAgent:   mockAgent,
	}

	// 5. Run QA Agent
	err = s.runQAAgent(context.Background())
	if err == nil {
		t.Error("Expected error from runQAAgent, got nil")
	}

	// Verify failure signal was set (or check it's 'false' depending on impl)
	val, err := store.GetSignal(projectName, "QA_PASSED")
	if err != nil {
		t.Errorf("Failed to get signal: %v", err)
	}
	if val != "false" {
		t.Errorf("Expected QA_PASSED signal 'false', got '%s'", val)
	}
}
