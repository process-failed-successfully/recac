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
	tempDir, err := os.MkdirTemp("", "state_mem_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	stateFile := filepath.Join(tempDir, "state.json")
	sm := NewStateManager(stateFile)

	err = sm.AddMemory("Learned something new")
	if err != nil {
		t.Fatalf("failed to add memory: %v", err)
	}

	state, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if len(state.Memory) != 1 {
		t.Errorf("expected 1 memory item, got %d", len(state.Memory))
	}
	if state.Memory[0] != "Learned something new" {
		t.Errorf("unexpected memory content: %s", state.Memory[0])
	}

	// Add another
	err = sm.AddMemory("Another thing")
	if err != nil {
		t.Fatalf("failed to add second memory: %v", err)
	}

	state, _ = sm.Load()
	if len(state.Memory) != 2 {
		t.Errorf("expected 2 memory items, got %d", len(state.Memory))
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

	// Init with defaults
	err = sm.InitializeState(1000, "gpt-4")
	if err != nil {
		t.Fatalf("failed to init state: %v", err)
	}

	state, err := sm.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if state.MaxTokens != 1000 {
		t.Errorf("expected MaxTokens 1000, got %d", state.MaxTokens)
	}
	if state.Model != "gpt-4" {
		t.Errorf("expected Model gpt-4, got %s", state.Model)
	}

	// Re-init shouldn't change existing if not 0?
	// The implementation says:
	// if state.MaxTokens == 0 && maxTokens > 0 { state.MaxTokens = maxTokens }
	// if state.Model == "" && model != "" { state.Model = model }

	err = sm.InitializeState(2000, "gpt-5")
	if err != nil {
		t.Fatalf("failed to re-init state: %v", err)
	}

	state, _ = sm.Load()
	// Should stay 1000 and gpt-4 because they were already set
	if state.MaxTokens != 1000 {
		t.Errorf("expected MaxTokens to remain 1000, got %d", state.MaxTokens)
	}
	if state.Model != "gpt-4" {
		t.Errorf("expected Model to remain gpt-4, got %s", state.Model)
	}
}
