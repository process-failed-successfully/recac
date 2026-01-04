package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateManager_AtomicSave(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "state_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	sm := NewStateManager(stateFile)

	state := State{
		CurrentTokens: 100,
	}

	if err := sm.Save(state); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Errorf("state file was not created")
	}

	// Verify content
	loadedState, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if loadedState.CurrentTokens != 100 {
		t.Errorf("expected 100 tokens, got %d", loadedState.CurrentTokens)
	}
}

func TestStateManager_CorruptLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "state_test_corrupt")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	sm := NewStateManager(stateFile)

	// Write garbage HTML
	garbage := "<html><body>Error</body></html>"
	if err := os.WriteFile(stateFile, []byte(garbage), 0644); err != nil {
		t.Fatalf("failed to write garbage: %v", err)
	}

	_, err = sm.Load()
	if err == nil {
		t.Fatal("expected error loading garbage state, got nil")
	}

	// Check error validation message
	// Expected: failed to unmarshal state (content starts with: "<html>..."): ...
	expectedSnippet := "failed to unmarshal state (content starts with: \"<html>"
	if len(err.Error()) < len(expectedSnippet) || err.Error()[:len(expectedSnippet)] != expectedSnippet {
		t.Errorf("error message mismatch. Got: %q, Expected start: %q", err.Error(), expectedSnippet)
	}
}
