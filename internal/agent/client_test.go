package agent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseClient_PreparePrompt(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	sm := NewStateManager(statePath)

	client := NewBaseClient("test-project", 100)
	client.StateManager = sm

	// Case 1: Initialize State
	prompt := "Hello"
	p, state, update, err := client.PreparePrompt(prompt)
	require.NoError(t, err)
	assert.Equal(t, "Hello", p)
	assert.True(t, update)
	// MaxTokens should default to DefaultMaxTokens (100) if not set in state
	// Wait, PreparePrompt logic:
	// if maxTokens == 0 { maxTokens = c.DefaultMaxTokens }
	// The loaded state has MaxTokens=0 initially.
	// But it does NOT save the default into state.MaxTokens unless we called InitializeState.
	// PreparePrompt uses local maxTokens variable.
	// And it updates state.CurrentTokens.
	assert.Greater(t, state.CurrentTokens, 0)

	// Case 2: Truncation
	// Create a long prompt > 50 tokens (50% of 100 is 50)
	// "word " is 1 token approx? EstimateTokenCount splits by space.
	longPrompt := strings.Repeat("word ", 60)
	p, state, update, err = client.PreparePrompt(longPrompt)
	require.NoError(t, err)
	assert.True(t, update)
	assert.True(t, len(p) < len(longPrompt))
	assert.Equal(t, 1, state.TokenUsage.TruncationCount)
}

func TestBaseClient_UpdateStateWithResponse(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	sm := NewStateManager(statePath)

	client := NewBaseClient("test-project", 1000)
	client.StateManager = sm

	// Initial State
	prompt := "Hello"
	_, state, _, _ := client.PreparePrompt(prompt)

	// Update
	response := "World"
	client.UpdateStateWithResponse(state, response)

	// Check persisted state
	loadedState, err := sm.Load()
	require.NoError(t, err)

	assert.Equal(t, 2, len(loadedState.History)) // User + Assistant
	assert.Equal(t, "user", loadedState.History[0].Role)
	assert.Equal(t, "assistant", loadedState.History[1].Role)
	assert.Equal(t, "World", loadedState.History[1].Content)

	// Check Token Usage
	assert.Greater(t, loadedState.TokenUsage.TotalTokens, 0)
	assert.Equal(t, 1.0, loadedState.Metadata["iteration"])
}

func TestBaseClient_SendWithRetry_Success(t *testing.T) {
	client := NewBaseClient("test-project", 1000)
	// No StateManager needed for basic SendWithRetry if PreparePrompt handles nil StateManager gracefully.
	// PreparePrompt: if c.StateManager == nil { return prompt, State{}, false, nil }
	// So it works without state manager.

	mockSend := func(ctx context.Context, p string) (string, error) {
		assert.Equal(t, "Hello", p)
		return "World", nil
	}

	resp, err := client.SendWithRetry(context.Background(), "Hello", mockSend)
	require.NoError(t, err)
	assert.Equal(t, "World", resp)
}

func TestBaseClient_SendWithRetry_RetryLogic(t *testing.T) {
	client := NewBaseClient("test-project", 1000)
	// Fast backoff for test
	client.BackoffFn = func(i int) time.Duration { return time.Millisecond }

	attempts := 0
	mockSend := func(ctx context.Context, p string) (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("transient error")
		}
		return "Success", nil
	}

	resp, err := client.SendWithRetry(context.Background(), "Hello", mockSend)
	require.NoError(t, err)
	assert.Equal(t, "Success", resp)
	assert.Equal(t, 3, attempts)
}

func TestBaseClient_SendWithRetry_Failure(t *testing.T) {
	client := NewBaseClient("test-project", 1000)
	client.BackoffFn = func(i int) time.Duration { return time.Millisecond }

	mockSend := func(ctx context.Context, p string) (string, error) {
		return "", errors.New("persistent error")
	}

	_, err := client.SendWithRetry(context.Background(), "Hello", mockSend)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 3 retries")
}

func TestBaseClient_SendStreamWithRetry_Success(t *testing.T) {
	client := NewBaseClient("test-project", 1000)

	mockSendStream := func(ctx context.Context, p string, onChunk func(string)) (string, error) {
		onChunk("A")
		onChunk("B")
		return "AB", nil
	}

	var chunks []string
	onChunk := func(c string) {
		chunks = append(chunks, c)
	}

	resp, err := client.SendStreamWithRetry(context.Background(), "Hello", mockSendStream, onChunk)
	require.NoError(t, err)
	assert.Equal(t, "AB", resp)
	assert.Equal(t, []string{"A", "B"}, chunks)
}

func TestBaseClient_SendStreamWithRetry_Retry(t *testing.T) {
	client := NewBaseClient("test-project", 1000)
	client.BackoffFn = func(i int) time.Duration { return time.Millisecond }

	attempts := 0
	mockSendStream := func(ctx context.Context, p string, onChunk func(string)) (string, error) {
		attempts++
		if attempts == 1 {
			return "", errors.New("stream error")
		}
		onChunk("C")
		return "C", nil
	}

	var chunks []string
	onChunk := func(c string) {
		chunks = append(chunks, c)
	}

	resp, err := client.SendStreamWithRetry(context.Background(), "Hello", mockSendStream, onChunk)
	require.NoError(t, err)
	assert.Equal(t, "C", resp)
	// NOTE: chunks might contain duplicates if the implementation didn't clear them,
	// BUT the client calls `onChunk` passed from `SendStreamWithRetry`.
	// The `onChunk` I passed appends to `chunks`.
	// The first attempt failed before calling onChunk? No, let's say it failed immediately.
	// If it called onChunk then failed, I'd see duplicates.
	// My mock didn't call onChunk on failure.
	assert.Equal(t, []string{"C"}, chunks)
}
