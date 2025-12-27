package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State represents the persistent state of an agent
type State struct {
	Memory       []string               `json:"memory"`
	History      []Message              `json:"history"`
	Metadata     map[string]interface{} `json:"metadata"`
	UpdatedAt    time.Time              `json:"updated_at"`
	MaxTokens    int                    `json:"max_tokens,omitempty"`    // Maximum token limit for context window
	CurrentTokens int                   `json:"current_tokens,omitempty"` // Current token count in context
	TokenUsage   TokenUsage             `json:"token_usage,omitempty"`   // Token usage statistics
}

// TokenUsage tracks token consumption statistics
type TokenUsage struct {
	TotalPromptTokens  int `json:"total_prompt_tokens"`  // Total tokens in prompts sent
	TotalResponseTokens int `json:"total_response_tokens"` // Total tokens in responses received
	TotalTokens        int `json:"total_tokens"`         // Total tokens used (prompt + response)
	TruncationCount    int `json:"truncation_count"`     // Number of times truncation occurred
}

// Message represents a chat message
type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// StateManager handles saving and loading state
type StateManager struct {
	FilePath string
	mu       sync.RWMutex
}

// NewStateManager creates a new state manager
func NewStateManager(filePath string) *StateManager {
	return &StateManager{
		FilePath: filePath,
	}
}

// Save writes the state to disk
func (sm *StateManager) Save(state State) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	state.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	dir := filepath.Dir(sm.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(sm.FilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// Load reads the state from disk
func (sm *StateManager) Load() (State, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var state State

		data, err := os.ReadFile(sm.FilePath)
		if os.IsNotExist(err) {
			// Return empty state if file doesn't exist
			return State{
				Memory:        []string{},
				History:       []Message{},
				Metadata:      make(map[string]interface{}),
				MaxTokens:     32000, // Default to 32k tokens (common for many models)
				CurrentTokens: 0,
				TokenUsage:    TokenUsage{},
			}, nil
		}
	if err != nil {
		return state, fmt.Errorf("failed to read state file: %w", err)
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return state, nil
}

// AddMemory adds a memory item to the state and saves it
func (sm *StateManager) AddMemory(memoryItem string) error {
	state, err := sm.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	
	state.Memory = append(state.Memory, memoryItem)
	
	return sm.Save(state)
}

// InitializeState initializes the state with max_tokens if not already set
func (sm *StateManager) InitializeState(maxTokens int) error {
	state, err := sm.Load()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	
	// Only set max_tokens if it's not already set (0 or uninitialized)
	if state.MaxTokens == 0 && maxTokens > 0 {
		state.MaxTokens = maxTokens
		return sm.Save(state)
	}
	
	return nil
}
