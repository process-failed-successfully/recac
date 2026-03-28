package runner

import (
	"context"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"testing"
    "os"
	"time"
)

func TestRunManagerAgent_Success(t *testing.T) {
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	projectID := "test-project"
	featureList := `{"project_name": "Test", "features": [{"id": "1", "description": "Test Feature", "status": "done", "passes": true}]}`
	if err := store.SaveFeatures(projectID, featureList); err != nil {
		t.Fatalf("Failed to save features: %v", err)
	}

	session := &Session{
		Workspace:    workspace,
		Project:      projectID,
		DBStore:      store,
		Agent:        &MockAgentForManager{Response: "I approve this."},
		ManagerAgent: &MockAgentForManager{Response: "I approve this."},
		Notifier:     notify.NewManager(func(string, ...interface{}) {}),
		Logger:       telemetry.NewLogger(true, "", false),
	}

	err := session.runManagerAgent(context.Background())
	if err != nil {
		t.Errorf("Expected nil error from runManagerAgent, got: %v", err)
	}
}

func TestRunManagerAgent_SignalSuccess(t *testing.T) {
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	projectID := "test-project"

	session := &Session{
		Workspace:    workspace,
		Project:      projectID,
		DBStore:      store,
		Agent:        &MockAgentForManager{Response: "I approve this."},
		ManagerAgent: &MockAgentForManager{Response: "I approve this."},
		Notifier:     notify.NewManager(func(string, ...interface{}) {}),
		Logger:       telemetry.NewLogger(true, "", false),
	}

	session.createSignal("PROJECT_SIGNED_OFF")

	err := session.runManagerAgent(context.Background())
	if err != nil {
		t.Errorf("Expected nil error from runManagerAgent, got: %v", err)
	}
}

func TestRunManagerAgent_InitAgent(t *testing.T) {
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

    os.Setenv("GEMINI_API_KEY", "test-key")
    defer os.Unsetenv("GEMINI_API_KEY")

	projectID := "test-project"

	session := &Session{
		Workspace:    workspace,
		Project:      projectID,
		DBStore:      store,
        AgentProvider: "gemini",
        AgentModel:    "gemini-1.5-pro-latest",
		Notifier:     notify.NewManager(func(string, ...interface{}) {}),
		Logger:       telemetry.NewLogger(true, "", false),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2 * time.Second)
	defer cancel()

	err := session.runManagerAgent(ctx)
	if err == nil {
		t.Errorf("Expected error from runManagerAgent")
	}
}

func TestRunQAAgent_InitAgent(t *testing.T) {
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

    os.Setenv("GEMINI_API_KEY", "test-key")
    defer os.Unsetenv("GEMINI_API_KEY")

	projectID := "test-project"

	session := &Session{
		Workspace:    workspace,
		Project:      projectID,
		DBStore:      store,
        AgentProvider: "gemini",
        AgentModel:    "gemini-1.5-flash-latest",
		Notifier:     notify.NewManager(func(string, ...interface{}) {}),
		Logger:       telemetry.NewLogger(true, "", false),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2 * time.Second)
	defer cancel()

	err := session.runQAAgent(ctx)
	if err == nil {
		t.Errorf("Expected error from runQAAgent")
	}
}

func TestRunManagerAgent_ConfigFallback(t *testing.T) {
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

    os.Setenv("OPENAI_API_KEY", "test-key")
    defer os.Unsetenv("OPENAI_API_KEY")

	projectID := "test-project"

	session := &Session{
		Workspace:    workspace,
		Project:      projectID,
		DBStore:      store,
        AgentProvider: "openai",
        AgentModel:    "gpt-4o-mini",
		Notifier:     notify.NewManager(func(string, ...interface{}) {}),
		Logger:       telemetry.NewLogger(true, "", false),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2 * time.Second)
	defer cancel()

	err := session.runManagerAgent(ctx)
	if err == nil {
		t.Errorf("Expected error from runManagerAgent")
	}
}

func TestRunManagerAgent_ConfigFallbackOpenRouter(t *testing.T) {
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

    os.Setenv("OPENROUTER_API_KEY", "test-key")
    defer os.Unsetenv("OPENROUTER_API_KEY")

	projectID := "test-project"

	session := &Session{
		Workspace:    workspace,
		Project:      projectID,
		DBStore:      store,
        AgentProvider: "openrouter",
        AgentModel:    "gpt-4o-mini",
		Notifier:     notify.NewManager(func(string, ...interface{}) {}),
		Logger:       telemetry.NewLogger(true, "", false),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2 * time.Second)
	defer cancel()

	err := session.runManagerAgent(ctx)
	if err == nil {
		t.Errorf("Expected error from runManagerAgent")
	}
}

func TestRunQAAgent_ConfigFallback(t *testing.T) {
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

    os.Setenv("OPENAI_API_KEY", "test-key")
    defer os.Unsetenv("OPENAI_API_KEY")

	projectID := "test-project"

	session := &Session{
		Workspace:    workspace,
		Project:      projectID,
		DBStore:      store,
        AgentProvider: "openai",
        AgentModel:    "gpt-4o-mini",
		Notifier:     notify.NewManager(func(string, ...interface{}) {}),
		Logger:       telemetry.NewLogger(true, "", false),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2 * time.Second)
	defer cancel()

	err := session.runQAAgent(ctx)
	if err == nil {
		t.Errorf("Expected error from runQAAgent")
	}
}

func TestRunQAAgent_ConfigFallbackOpenRouter(t *testing.T) {
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

    os.Setenv("OPENROUTER_API_KEY", "test-key")
    defer os.Unsetenv("OPENROUTER_API_KEY")

	projectID := "test-project"

	session := &Session{
		Workspace:    workspace,
		Project:      projectID,
		DBStore:      store,
        AgentProvider: "openrouter",
        AgentModel:    "gpt-4o-mini",
		Notifier:     notify.NewManager(func(string, ...interface{}) {}),
		Logger:       telemetry.NewLogger(true, "", false),
	}

	err := session.runQAAgent(context.Background())
	if err == nil {
		t.Errorf("Expected error from runQAAgent")
	}
}
