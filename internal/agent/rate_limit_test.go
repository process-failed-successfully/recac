package agent

import (
	"os"
	"testing"
	"time"
)

func TestNewBaseClient_RespectsAgentDelay(t *testing.T) {
	// Set the environment variable
	t.Setenv("RECAC_AGENT_DELAY", "10s")

	client := NewBaseClient("test-project", 32000)

	// Test retry 1 (normally 1s)
	backoff := client.BackoffFn(1)
	if backoff < 10*time.Second {
		t.Errorf("Expected backoff >= 10s, got %v", backoff)
	}

	// Test retry 3 (normally 4s)
	backoff = client.BackoffFn(3)
	if backoff < 10*time.Second {
		t.Errorf("Expected backoff >= 10s, got %v", backoff)
	}
}

func TestNewBaseClient_DefaultBackoff(t *testing.T) {
	// Ensure env var is unset
	os.Unsetenv("RECAC_AGENT_DELAY")

	client := NewBaseClient("test-project", 32000)

	// Test retry 1 (expected 1s)
	backoff := client.BackoffFn(1)
	if backoff != 1*time.Second {
		t.Errorf("Expected backoff 1s, got %v", backoff)
	}

	// Test retry 2 (expected 2s)
	backoff = client.BackoffFn(2)
	if backoff != 2*time.Second {
		t.Errorf("Expected backoff 2s, got %v", backoff)
	}
}
