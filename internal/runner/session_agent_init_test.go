package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"testing"
)

type MockAgentForInit struct {
	SendFunc       func(ctx context.Context, prompt string) (string, error)
	SendStreamFunc func(ctx context.Context, prompt string, onChunk func(string)) (string, error)
}

func (m *MockAgentForInit) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "", nil
}

func (m *MockAgentForInit) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if m.SendStreamFunc != nil {
		return m.SendStreamFunc(ctx, prompt, onChunk)
	}
	return "", nil
}

func TestRunQAAgent_Initialization(t *testing.T) {
	// 1. Setup
	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	// 2. Mock newAgentFunc - Success
	newAgentFunc = func(provider, apiKey, model, workspace, project string) (agent.Agent, error) {
		return &MockAgentForInit{
			SendFunc: func(ctx context.Context, prompt string) (string, error) {
				store.SetSignal(project, "QA_PASSED", "true")
				return "QA Passed", nil
			},
		}, nil
	}

	session := &Session{
		Workspace:     workspace,
		Project:       "test-project",
		AgentProvider: "test-provider",
		AgentModel:    "test-model",
		DBStore:       store,
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Logger:        telemetry.NewLogger(true, "", false),
	}

	// 3. Test Success Path
	if err := session.runQAAgent(context.Background()); err != nil {
		t.Errorf("runQAAgent failed initialization: %v", err)
	}

	// 4. Mock newAgentFunc - Failure
	newAgentFunc = func(provider, apiKey, model, workspace, project string) (agent.Agent, error) {
		return nil, fmt.Errorf("init error")
	}

	session.QAAgent = nil // Reset
	if err := session.runQAAgent(context.Background()); err == nil {
		t.Error("Expected error from runQAAgent initialization failure")
	}
}

func TestRunManagerAgent_Initialization(t *testing.T) {
	// 1. Setup
	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	// 2. Mock newAgentFunc - Success
	newAgentFunc = func(provider, apiKey, model, workspace, project string) (agent.Agent, error) {
		return &MockAgentForInit{
			SendFunc: func(ctx context.Context, prompt string) (string, error) {
				store.SetSignal(project, "PROJECT_SIGNED_OFF", "true")
				return "Manager Approved", nil
			},
		}, nil
	}

	session := &Session{
		Workspace:     workspace,
		Project:       "test-project",
		AgentProvider: "test-provider",
		AgentModel:    "test-model",
		DBStore:       store,
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Logger:        telemetry.NewLogger(true, "", false),
	}

	// 3. Test Success Path
	if err := session.runManagerAgent(context.Background()); err != nil {
		t.Errorf("runManagerAgent failed initialization: %v", err)
	}

	// 4. Mock newAgentFunc - Failure
	newAgentFunc = func(provider, apiKey, model, workspace, project string) (agent.Agent, error) {
		return nil, fmt.Errorf("init error")
	}

	session.ManagerAgent = nil // Reset
	if err := session.runManagerAgent(context.Background()); err == nil {
		t.Error("Expected error from runManagerAgent initialization failure")
	}
}
