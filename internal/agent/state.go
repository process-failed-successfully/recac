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
	Model         string                 `json:"model,omitempty"`          // Model used by the agent
	Memory        []string               `json:"memory"`
	History       []Message              `json:"history"`
	Metadata      map[string]interface{} `json:"metadata"`
	UpdatedAt     time.Time              `json:"updated_at"`
	MaxTokens     int                    `json:"max_tokens,omitempty"`     // Maximum token limit for context window
	CurrentTokens int                    `json:"current_tokens,omitempty"` // Current token count in context
	TokenUsage    TokenUsage             `json:"token_usage,omitempty"`    // Token usage statistics
}

// TokenUsage tracks token consumption statistics
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`     // Total tokens in prompts sent
	CompletionTokens int `json:"completion_tokens"` // Total tokens in responses received
	TotalTokens      int `json:"total_tokens"`      // Total tokens used (prompt + response)
	TruncationCount  int `json:"truncation_count"`  // Number of times truncation occurred
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

// loadState reads the state from disk (internal, no lock)
func (sm *StateManager) loadState() (State, error) {
	var state State

	data, err := os.ReadFile(sm.FilePath)
	if os.IsNotExist(err) {
		// Return empty state if file doesn't exist
		// Note: MaxTokens defaults to 0, which means "uninitialized"
		return State{
			Memory:        []string{},
			History:       []Message{},
			Metadata:      make(map[string]interface{}),
			CurrentTokens: 0,
			TokenUsage:    TokenUsage{},
		}, nil
	}
	if err != nil {
		return state, fmt.Errorf("failed to read state file: %w", err)
	}

	if err := json.Unmarshal(data, &state); err != nil {
		snippet := string(data)
		if len(snippet) > 100 {
			snippet = snippet[:100] + "..."
		}
		// Return specific error with snippet to help debugging
		return state, fmt.Errorf("failed to unmarshal state (content starts with: %q): %w", snippet, err)
	}

	return state, nil
}

// saveState writes the state to disk (internal, no lock)
func (sm *StateManager) saveState(state State) error {
	state.UpdatedAt = time.Now()

	// Truncate history to avoid infinite growth and context overflow
	const maxHistoryEntries = 50
	if len(state.History) > maxHistoryEntries {
		state.History = state.History[len(state.History)-maxHistoryEntries:]
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	dir := filepath.Dir(sm.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Atomic write: write to temp file then rename
	tmpPath := sm.FilePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	if err := os.Rename(tmpPath, sm.FilePath); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

// Save writes the state to disk
func (sm *StateManager) Save(state State) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.saveState(state)
}

// Load reads the state from disk
func (sm *StateManager) Load() (State, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.loadState()
}

// AddMemory adds a memory item to the state and saves it
func (sm *StateManager) AddMemory(memoryItem string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	state, err := sm.loadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	state.Memory = append(state.Memory, memoryItem)

	return sm.saveState(state)
}

// InitializeState initializes the state with max_tokens if not already set
func (sm *StateManager) InitializeState(maxTokens int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	state, err := sm.loadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Only set max_tokens if it's not already set (0)
	if state.MaxTokens == 0 && maxTokens > 0 {
		state.MaxTokens = maxTokens
		return sm.saveState(state)
	}

	return nil
}
