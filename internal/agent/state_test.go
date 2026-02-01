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

func TestStateManager_AddMemory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "state_test_memory")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	sm := NewStateManager(stateFile)

	// Initial save
	initialState := State{Memory: []string{"Item 1"}}
	if err := sm.Save(initialState); err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	// Add memory
	if err := sm.AddMemory("Item 2"); err != nil {
		t.Fatalf("failed to add memory: %v", err)
	}

	// Verify
	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if len(loaded.Memory) != 2 {
		t.Errorf("expected 2 memory items, got %d", len(loaded.Memory))
	}
	if loaded.Memory[1] != "Item 2" {
		t.Errorf("expected 'Item 2', got %q", loaded.Memory[1])
	}
}

func TestStateManager_InitializeState(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "state_test_init")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	sm := NewStateManager(stateFile)

	// Test 1: Initialize fresh state
	if err := sm.InitializeState(1000, "gpt-4"); err != nil {
		t.Fatalf("failed to initialize state: %v", err)
	}

	loaded, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if loaded.MaxTokens != 1000 {
		t.Errorf("expected MaxTokens 1000, got %d", loaded.MaxTokens)
	}
	if loaded.Model != "gpt-4" {
		t.Errorf("expected Model 'gpt-4', got %q", loaded.Model)
	}

	// Test 2: Initialize existing state (should not overwrite if already set)
	if err := sm.InitializeState(2000, "gpt-3.5"); err != nil {
		t.Fatalf("failed to initialize state again: %v", err)
	}

	loaded, err = sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	// Should remain unchanged
	if loaded.MaxTokens != 1000 {
		t.Errorf("expected MaxTokens to remain 1000, got %d", loaded.MaxTokens)
	}
	if loaded.Model != "gpt-4" {
		t.Errorf("expected Model to remain 'gpt-4', got %q", loaded.Model)
	}
}

func TestLoadState_Helper(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "state_test_load")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")

	// Create a dummy state file
	sm := NewStateManager(stateFile)
	if err := sm.Save(State{Model: "test-model"}); err != nil {
		t.Fatalf("failed to save setup state: %v", err)
	}

	// Use LoadState helper
	state, err := LoadState(stateFile)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.Model != "test-model" {
		t.Errorf("expected model 'test-model', got %q", state.Model)
	}
}
