package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateManager(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, ".agent_state.json")
	sm := NewStateManager(stateFile)

	// Test 1: Load non-existent file
	state, err := sm.Load()
	if err != nil {
		t.Fatalf("Load on non-existent file should not error: %v", err)
	}
	if len(state.Memory) != 0 {
		t.Errorf("Expected empty memory, got %v", state.Memory)
	}

	// Test 2: Save state
	testState := State{
		Memory: []string{"Fact 1", "Fact 2"},
		History: []Message{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
			{Role: "assistant", Content: "Hi there", Timestamp: time.Now()},
		},
		Metadata: map[string]interface{}{
			"iteration": 1.0, // JSON numbers are float64
		},
	}

	err = sm.Save(testState)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Check if file exists
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Fatalf("State file was not created at %s", stateFile)
	}

	// Test 3: Load saved state
	loadedState, err := sm.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loadedState.Memory) != 2 {
		t.Errorf("Expected 2 memory items, got %d", len(loadedState.Memory))
	}
	if loadedState.Memory[0] != "Fact 1" {
		t.Errorf("Expected 'Fact 1', got '%s'", loadedState.Memory[0])
	}
	if len(loadedState.History) != 2 {
		t.Errorf("Expected 2 history items, got %d", len(loadedState.History))
	}
	if loadedState.Metadata["iteration"] != 1.0 {
		t.Errorf("Expected iteration 1.0, got %v", loadedState.Metadata["iteration"])
	}
}

// TestStateManager_AgentMemoryPersistence verifies Feature #14:
// "Verify the agent state is persisted to disk (.agent_state.json)"
func TestStateManager_AgentMemoryPersistence(t *testing.T) {
	// Step 1: Setup - Create a temporary workspace to simulate a session
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, ".agent_state.json")
	sm := NewStateManager(stateFile)

	// Step 2: Simulate a session that updates agent memory
	// In a real session, the agent would learn facts and add them to memory
	memoryItems := []string{
		"The project uses Go 1.21+",
		"The main entry point is cmd/recac/main.go",
		"Configuration is loaded from config.yaml",
	}

	for _, item := range memoryItems {
		if err := sm.AddMemory(item); err != nil {
			t.Fatalf("Failed to add memory item '%s': %v", item, err)
		}
	}

	// Step 3: Stop the session (simulated by finishing the test)
	// Verify .agent_state.json exists
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Fatalf("State file was not created at %s", stateFile)
	}

	// Step 4: Check that .agent_state.json contains the memory data
	loadedState, err := sm.Load()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if len(loadedState.Memory) != len(memoryItems) {
		t.Errorf("Expected %d memory items, got %d", len(memoryItems), len(loadedState.Memory))
	}

	for i, expected := range memoryItems {
		if i >= len(loadedState.Memory) {
			t.Errorf("Memory item %d is missing", i)
			continue
		}
		if loadedState.Memory[i] != expected {
			t.Errorf("Memory item %d: expected '%s', got '%s'", i, expected, loadedState.Memory[i])
		}
	}

	// Verify the file contains JSON with memory data
	fileContent, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	var fileState State
	if err := json.Unmarshal(fileContent, &fileState); err != nil {
		t.Fatalf("State file does not contain valid JSON: %v", err)
	}

	if len(fileState.Memory) != len(memoryItems) {
		t.Errorf("File state has %d memory items, expected %d", len(fileState.Memory), len(memoryItems))
	}
}

func TestStateManager_InitializeState(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, ".agent_state.json")
	sm := NewStateManager(stateFile)

	// Test 1: Initialize with max tokens
	if err := sm.InitializeState(1000); err != nil {
		t.Fatalf("InitializeState failed: %v", err)
	}

	state, err := sm.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.MaxTokens != 1000 {
		t.Errorf("Expected MaxTokens 1000, got %d", state.MaxTokens)
	}

	// Test 2: Initialize again should NOT overwrite (if existing is non-zero)
	// But wait, the logic says: "Only set max_tokens if it's not already set (0 or uninitialized)"
	// So calling it again with a different value should do nothing.
	if err := sm.InitializeState(2000); err != nil {
		t.Fatal(err)
	}
	
	state, err = sm.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.MaxTokens != 1000 {
		t.Errorf("Expected MaxTokens to remain 1000, got %d", state.MaxTokens)
	}
}

func TestStateManager_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, ".agent_state.json")
	sm := NewStateManager(stateFile)
	
	concurrency := 10
	done := make(chan bool)

	// Initialize first
	sm.InitializeState(1000)

	// Launch concurrent writers
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			sm.AddMemory(fmt.Sprintf("Memory %d", id))
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < concurrency; i++ {
		<-done
	}

	state, err := sm.Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(state.Memory) != concurrency {
		t.Errorf("Expected %d memory items, got %d", concurrency, len(state.Memory))
	}
}
