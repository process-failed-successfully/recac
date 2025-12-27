package agent

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestGeminiClient_Retry(t *testing.T) {
	calls := 0
	client := NewGeminiClient("fake-key", "gemini-pro")
	client.WithMockResponder(func(prompt string) (string, error) {
		calls++
		if calls < 3 {
			return "", fmt.Errorf("temporary error")
		}
		return "Success", nil
	})

	ctx := context.Background()
	result, err := client.Send(ctx, "test prompt")
	
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	
	if result != "Success" {
		t.Errorf("expected 'Success', got %q", result)
	}
	
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

// TestGeminiClient_NetworkInterruption verifies handling of network interruptions
// Step 1: Simulate network drop during agent call
// Step 2: Verify the application pauses or retries
// Step 3: Verify it does not crash
func TestGeminiClient_NetworkInterruption(t *testing.T) {
	// Test case 1: Network error that eventually succeeds after retries
	t.Run("NetworkDrop_RecoversAfterRetries", func(t *testing.T) {
		calls := 0
		client := NewGeminiClient("fake-key", "gemini-pro")
		
		// Simulate network drop (connection refused, timeout, etc.)
		client.WithMockResponder(func(prompt string) (string, error) {
			calls++
			if calls == 1 {
				// First call: simulate network connection error
				return "", fmt.Errorf("dial tcp: connection refused")
			}
			if calls == 2 {
				// Second call: simulate timeout error
				return "", fmt.Errorf("context deadline exceeded")
			}
			if calls == 3 {
				// Third call: simulate temporary network error
				return "", fmt.Errorf("read: connection reset by peer")
			}
			// Fourth call: network recovers, request succeeds
			return "Success after network recovery", nil
		})

		ctx := context.Background()
		startTime := time.Now()
		
		// Verify it doesn't crash (no panic)
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Application crashed with panic: %v", r)
			}
		}()
		
		result, err := client.Send(ctx, "test prompt")
		
		// Verify retries happened (should have called 4 times)
		if calls != 4 {
			t.Errorf("expected 4 calls (3 failures + 1 success), got %d", calls)
		}
		
		// Verify eventual success
		if err != nil {
			t.Fatalf("expected success after retries, got error: %v", err)
		}
		
		if result != "Success after network recovery" {
			t.Errorf("expected 'Success after network recovery', got %q", result)
		}
		
		// Verify exponential backoff was applied (should take some time)
		elapsed := time.Since(startTime)
		if elapsed < 3*time.Second {
			t.Errorf("expected retries with backoff to take at least 3 seconds, took %v", elapsed)
		}
	})

	// Test case 2: Network error that fails after max retries (but doesn't crash)
	t.Run("NetworkDrop_FailsGracefully", func(t *testing.T) {
		calls := 0
		client := NewGeminiClient("fake-key", "gemini-pro")
		
		// Simulate persistent network failure
		client.WithMockResponder(func(prompt string) (string, error) {
			calls++
			// Always return network error
			return "", fmt.Errorf("dial tcp: no route to host")
		})

		ctx := context.Background()
		
		// Verify it doesn't crash (no panic)
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Application crashed with panic: %v", r)
			}
		}()
		
		result, err := client.Send(ctx, "test prompt")
		
		// Verify retries happened (maxRetries = 3, so 4 total attempts: initial + 3 retries)
		expectedCalls := 4
		if calls != expectedCalls {
			t.Errorf("expected %d calls (1 initial + 3 retries), got %d", expectedCalls, calls)
		}
		
		// Verify it returns an error (not a panic)
		if err == nil {
			t.Error("expected error after max retries, got nil")
		}
		
		// Verify error message indicates retry exhaustion
		if result != "" {
			t.Errorf("expected empty result on failure, got %q", result)
		}
		
		// Verify error message mentions retries
		if err != nil && err.Error() == "" {
			t.Error("expected non-empty error message")
		}
	})

	// Test case 3: Context cancellation during network interruption
	t.Run("NetworkDrop_ContextCancellation", func(t *testing.T) {
		calls := 0
		client := NewGeminiClient("fake-key", "gemini-pro")
		
		client.WithMockResponder(func(prompt string) (string, error) {
			calls++
			return "", fmt.Errorf("dial tcp: connection refused")
		})

		ctx, cancel := context.WithCancel(context.Background())
		
		// Cancel context after a short delay
		go func() {
			time.Sleep(500 * time.Millisecond)
			cancel()
		}()
		
		// Verify it doesn't crash
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Application crashed with panic: %v", r)
			}
		}()
		
		result, err := client.Send(ctx, "test prompt")
		
		// Verify context cancellation is handled gracefully
		if err == nil {
			t.Error("expected error due to context cancellation, got nil")
		}
		
		// Verify it doesn't continue retrying after context is cancelled
		// (should stop after context is cancelled, not after max retries)
		if calls > 2 {
			t.Errorf("expected to stop after context cancellation, but made %d calls", calls)
		}
		
		if result != "" {
			t.Errorf("expected empty result on cancellation, got %q", result)
		}
	})
}