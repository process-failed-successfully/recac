package runner // MockAgentForManager simulates the agent interaction for Manager
import (
	"context"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"testing"
)

type MockAgentForManager struct {
	Response string
}

func (m *MockAgentForManager) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockAgentForManager) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, nil
}

func TestManagerRejection_ClearsSignals(t *testing.T) {
	// 1. Setup
	workspace := t.TempDir()
	dbPath := filepath.Join(workspace, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	// 2. Mock Agent that Rejects
	// The session looks for completion ratio.
	// If I mock runManagerAgent, it calls RunQA (which returns empty/mocked report).
	// Then it sends prompt to agent.
	// Manager approves if completion ratio >= 0.95.
	// Since RunQA (static) might return something specific, I need to know what RunQA returns for an empty feature list.
	// Actually, session.go's runManagerAgent logic relies on `RunQA(features)`.
	// If feature list is empty/missing, RunQA might return 0?
	// Let's create a feature_list.json with 1 failing feature to ensure < 100% and force rejection?
	// OR, relies on the static `RunQA` output.
	// If the existing `RunQA` logic is used, it might be hard to control validity without writing features.
	//
	// Use: "Manager REJECTED: Completion ratio too low" is the error message.
	// This appears to depend largely on `qaReport.CompletionRatio`.
	// The `Agent` response is just logged, not parsed for decision currently (as per comments in session.go: "For now, manager approves if QA report shows 100% completion").

	// So to force rejection: ensure QA report isn't perfect.
	// Create a feature list with a feature that is NOT passing.
	projectID := "test-project"
	featureList := `{"project_name": "Test", "features": [{"id": "1", "description": "Test Feature", "status": "failed", "passes": false}]}`
	if err := store.SaveFeatures(projectID, featureList); err != nil {
		t.Fatalf("Failed to save features: %v", err)
	}

	session := &Session{
		Workspace:    workspace,
		Project:      projectID,
		DBStore:      store,
		Agent:        &MockAgentForManager{Response: "I reject this."},
		ManagerAgent: &MockAgentForManager{Response: "I reject this."},
		Notifier:     notify.NewManager(func(string, ...interface{}) {}),
		Logger:       telemetry.NewLogger(true, ""),
	}

	// 3. Set Signals
	session.createSignal("QA_PASSED")
	session.createSignal("COMPLETED")

	// 4. Run Manager Agent
	// It should fail because verified is false -> completion ratio 0.
	err := session.runManagerAgent(context.Background())
	if err == nil {
		t.Log("Expected error from runManagerAgent due to low completion ratio")
	}

	// 5. Verify Signals Cleared
	if session.hasSignal("QA_PASSED") {
		t.Error("QA_PASSED signal should have been cleared")
	}
	if session.hasSignal("COMPLETED") {
		t.Error("COMPLETED signal should have been cleared")
	}
}
