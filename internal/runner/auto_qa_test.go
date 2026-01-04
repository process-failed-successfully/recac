package runner

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"recac/internal/db"
	"testing"
)

func TestSession_CheckAutoQA(t *testing.T) {
	workspace := t.TempDir()

	// Create Mock DB Store to check signals
	dbPath := filepath.Join(workspace, ".recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB store: %v", err)
	}
	defer store.Close()

	s := &Session{
		Workspace: workspace,
		DBStore:   store,
		Logger:    slog.Default(),
	}

	// 1. No features -> False
	if s.checkAutoQA() {
		t.Error("Expected false for no features")
	}

	// 2. Some passing -> False
	writeFeaturesForAutoQAWithSession(t, s, []db.Feature{
		{Category: "Mixed", Status: "done"},
		{Category: "Mixed", Status: "pending"},
	})
	if s.checkAutoQA() {
		t.Error("Expected false for mixed features")
	}

	// 3. All passing -> True
	writeFeaturesForAutoQAWithSession(t, s, []db.Feature{
		{Category: "AllPass", Status: "done"},
		{Category: "AllPass", Status: "done"},
	})
	if !s.checkAutoQA() {
		t.Error("Expected true for all passing")
	}

	// Verify signal COMPLETED created
	val, _ := store.GetSignal("COMPLETED")
	if val != "true" {
		t.Error("Expected COMPLETED signal to be created")
	}

	// 4. Already signaled -> False
	if s.checkAutoQA() {
		t.Error("Expected false if already signaled")
	}

	// Reset signal
	store.DeleteSignal("COMPLETED")

	// 5. Existing signals -> False
	store.SetSignal("QA_PASSED", "true")
	if s.checkAutoQA() {
		t.Error("Expected false if QA_PASSED exists")
	}
}
func writeFeaturesForAutoQAWithSession(t *testing.T, s *Session, features []db.Feature) {
	list := db.FeatureList{
		ProjectName: "Test Project",
		Features:    features,
	}
	data, _ := json.Marshal(list)
	os.WriteFile(filepath.Join(s.Workspace, "feature_list.json"), data, 0644)
	if s.DBStore != nil {
		_ = s.DBStore.SaveFeatures(string(data))
	}
}
