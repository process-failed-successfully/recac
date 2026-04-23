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

func TestStateManager_Memory(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")
	sm := NewStateManager(stateFile)

	err := sm.AddMemory("test memory 1")
	if err != nil {
		t.Fatalf("failed to add memory: %v", err)
	}

	err = sm.AddMemory("test memory 2")
	if err != nil {
		t.Fatalf("failed to add memory: %v", err)
	}

	state, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if len(state.Memory) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(state.Memory))
	}
	if state.Memory[0] != "test memory 1" {
		t.Fatalf("expected first memory to be 'test memory 1', got %s", state.Memory[0])
	}

	err = sm.ClearMemory()
	if err != nil {
		t.Fatalf("failed to clear memory: %v", err)
	}

	state, err = sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if len(state.Memory) != 0 {
		t.Fatalf("expected 0 memories, got %d", len(state.Memory))
	}
}

func TestStateManager_LoadSave_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")
	sm := NewStateManager(stateFile)

	// Test invalid JSON on load
	statePath := stateFile
	err := os.MkdirAll(filepath.Dir(statePath), 0755)
	if err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	err = os.WriteFile(statePath, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err = sm.loadState()
	if err == nil {
		t.Fatalf("expected error on invalid JSON, got none")
	}

	// Test mkdir error on save
	os.RemoveAll(filepath.Dir(statePath))
	err = os.WriteFile(filepath.Dir(statePath), []byte("file"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err = sm.saveState(State{})
	if err == nil {
		t.Fatalf("expected error on save due to mkdir failure, got none")
	}
}

func TestStateManager_InitializeState(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "state_init_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	sm := NewStateManager(stateFile)

	// Test Initializing empty state
	err = sm.InitializeState(1000, "test-model")
	if err != nil {
		t.Fatalf("failed to initialize state: %v", err)
	}

	state, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if state.MaxTokens != 1000 {
		t.Errorf("expected MaxTokens to be 1000, got %d", state.MaxTokens)
	}
	if state.Model != "test-model" {
		t.Errorf("expected Model to be test-model, got %s", state.Model)
	}

	// Test Initializing an already initialized state (should not override)
	err = sm.InitializeState(2000, "new-model")
	if err != nil {
		t.Fatalf("failed to initialize state: %v", err)
	}

	state, err = sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if state.MaxTokens != 1000 {
		t.Errorf("expected MaxTokens to remain 1000, got %d", state.MaxTokens)
	}
	if state.Model != "test-model" {
		t.Errorf("expected Model to remain test-model, got %s", state.Model)
	}
}
