package runner

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/db"
	"strings"
	"testing"
)

// MockAgentForQA simulates the agent interaction for QA
type MockAgentForQA struct {
	Response  string
	Workspace string
}

func (m *MockAgentForQA) Send(ctx context.Context, prompt string) (string, error) {
	// Simulate agent action: write result file
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		resultPath := filepath.Join(m.Workspace, ".qa_result")
		os.WriteFile(resultPath, []byte(m.Response), 0644)
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
		Response:  "PASS",
		Workspace: workspace,
	}

	// 4. Create Session
	session := &Session{
		Agent:     mockAgent,
		QAAgent:   mockAgent,
		Workspace: workspace,
		DBStore:   store,
		// Docker client not needed for this test
		Docker: nil,
	}

	// 5. Run QA Agent
	err = session.runQAAgent(context.Background())
	if err != nil {
		t.Errorf("runQAAgent failed: %v", err)
	}

	// 6. Verify Signal
	val, err := store.GetSignal("QA_PASSED")
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
		Response:  "FAIL",
		Workspace: workspace,
	}

	// 4. Create Session
	session := &Session{
		Agent:     mockAgent,
		QAAgent:   mockAgent,
		Workspace: workspace,
		DBStore:   store,
	}

	// 5. Run QA Agent
	err = session.runQAAgent(context.Background())
	if err == nil {
		t.Error("Expected error from runQAAgent, got nil")
	}

	// 6. Verify Signal (Should NOT be present)
	val, err := store.GetSignal("QA_PASSED")
	if err != nil {
		t.Errorf("Failed to get signal: %v", err)
	}
	if val != "" {
		t.Errorf("Expected empty QA_PASSED signal, got '%s'", val)
	}
}
