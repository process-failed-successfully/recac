package agent

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBaseClient_PreparePrompt(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	sm := NewStateManager(stateFile)
	// Initialize state
	err := sm.InitializeState(100, "test-model")
	if err != nil {
		t.Fatalf("Failed to initialize state: %v", err)
	}

	client := NewBaseClient("test-project", 100)
	client.StateManager = sm

	t.Run("Normal Prompt", func(t *testing.T) {
		prompt := "Hello"
		prepared, state, shouldUpdate, err := client.PreparePrompt(prompt)
		assert.NoError(t, err)
		assert.True(t, shouldUpdate)
		assert.Equal(t, prompt, prepared)
		assert.Equal(t, 1, len(state.History))
		assert.Equal(t, prompt, state.History[0].Content)
		// "Hello" is 5 chars. EstimateTokenCount: 5/4 + 1 = 2 tokens.
		assert.Equal(t, 2, state.CurrentTokens)
	})

	t.Run("Truncated Prompt", func(t *testing.T) {
		// Limit is 100. Reserve 50% = 50 tokens.
		// If prompt exceeds 50 tokens, it should truncate.
		// 50 tokens * 4 chars = 200 chars approximately.
		longPrompt := makeString(300) // ~75 tokens

		prepared, state, _, err := client.PreparePrompt(longPrompt)
		assert.NoError(t, err)
		assert.NotEqual(t, longPrompt, prepared)
		assert.Contains(t, prepared, "truncated")
		assert.True(t, state.TokenUsage.TruncationCount > 0)
	})
}

func TestBaseClient_UpdateStateWithResponse(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state.json")

	sm := NewStateManager(stateFile)
	sm.InitializeState(1000, "test-model")

	client := NewBaseClient("test-project", 1000)
	client.StateManager = sm

	// Load initial state
	state, _ := sm.Load()
	state.CurrentTokens = 10
	state.TokenUsage.TotalPromptTokens = 10

	response := "World"
	client.UpdateStateWithResponse(state, response)

	// Reload state to verify persistence
	loadedState, err := sm.Load()
	assert.NoError(t, err)

	assert.Equal(t, 1, len(loadedState.History))
	assert.Equal(t, "assistant", loadedState.History[0].Role)
	assert.Equal(t, response, loadedState.History[0].Content)

	// "World" is 5 chars -> 2 tokens.
	// Initial current: 10. +2 = 12.
	assert.Equal(t, 12, loadedState.CurrentTokens)
	assert.Equal(t, 12, loadedState.TokenUsage.TotalTokens)
	assert.Equal(t, 2, loadedState.TokenUsage.TotalResponseTokens)
	assert.Equal(t, 1.0, loadedState.Metadata["iteration"])
}

func TestBaseClient_SendWithRetry(t *testing.T) {
	client := NewBaseClient("test-project", 1000)
	// Mock Backoff to be instant
	client.BackoffFn = func(i int) time.Duration { return 0 }
	// No StateManager for this test to isolate retry logic (or we can add one if needed)

	t.Run("Success First Try", func(t *testing.T) {
		calls := 0
		resp, err := client.SendWithRetry(context.Background(), "prompt", func(ctx context.Context, p string) (string, error) {
			calls++
			return "response", nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "response", resp)
		assert.Equal(t, 1, calls)
	})

	t.Run("Success After Retry", func(t *testing.T) {
		calls := 0
		resp, err := client.SendWithRetry(context.Background(), "prompt", func(ctx context.Context, p string) (string, error) {
			calls++
			if calls < 3 {
				return "", errors.New("temp error")
			}
			return "response", nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "response", resp)
		assert.Equal(t, 3, calls)
	})

	t.Run("Fail Max Retries", func(t *testing.T) {
		calls := 0
		resp, err := client.SendWithRetry(context.Background(), "prompt", func(ctx context.Context, p string) (string, error) {
			calls++
			return "", errors.New("persistent error")
		})
		assert.Error(t, err)
		assert.Equal(t, "", resp)
		assert.Equal(t, 4, calls) // Initial + 3 retries
	})
}

func TestBaseClient_SendStreamWithRetry(t *testing.T) {
	client := NewBaseClient("test-project", 1000)
	client.BackoffFn = func(i int) time.Duration { return 0 }

	t.Run("Success Streaming", func(t *testing.T) {
		chunks := []string{"Hello", " ", "World"}
		received := ""

		resp, err := client.SendStreamWithRetry(context.Background(), "prompt", func(ctx context.Context, p string, onChunk func(string)) (string, error) {
			for _, c := range chunks {
				onChunk(c)
			}
			return "Hello World", nil
		}, func(chunk string) {
			received += chunk
		})

		assert.NoError(t, err)
		assert.Equal(t, "Hello World", resp)
		assert.Equal(t, "Hello World", received)
	})

	t.Run("Retry Streaming", func(t *testing.T) {
		calls := 0
		received := ""

		resp, err := client.SendStreamWithRetry(context.Background(), "prompt", func(ctx context.Context, p string, onChunk func(string)) (string, error) {
			calls++
			if calls == 1 {
				onChunk("Bad")
				return "", errors.New("fail")
			}
			onChunk("Good")
			return "Good", nil
		}, func(chunk string) {
			received += chunk
		})

		assert.NoError(t, err)
		assert.Equal(t, "Good", resp)
		// Note: "Bad" might be in received depending on implementation.
		// The current implementation calls onChunk for the failed attempt too.
		// "BadGood"
		assert.Contains(t, received, "Good")
		assert.Equal(t, 2, calls)
	})
}

func makeString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
