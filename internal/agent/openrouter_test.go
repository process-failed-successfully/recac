package agent

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockRoundTripper implements http.RoundTripper
type MockRoundTripper struct {
	RoundTripFunc func(req *http.Request) *http.Response
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req), nil
}

func TestOpenRouterClient(t *testing.T) {
	client := NewOpenRouterClient("dummy-key", "test-model", "test-project")

	// 1. Test Send with Mock Responder (Bypasses HTTP)
	mockResponse := "Mocked OpenRouter Response"
	client.WithMockResponder(func(prompt string) (string, error) {
		if prompt == "fail" {
			return "", context.DeadlineExceeded
		}
		return mockResponse, nil
	})

	resp, err := client.Send(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if resp != mockResponse {
		t.Errorf("Expected '%s', got '%s'", mockResponse, resp)
	}

	// 2. Test State Manager integration
	tmpDir := t.TempDir()
	stateFile := tmpDir + "/agent_state.json"
	sm := NewStateManager(stateFile)
	_ = sm.InitializeState(1000, "test-model")

	client.WithStateManager(sm)

	resp, err = client.Send(context.Background(), "Hello with State")
	if err != nil {
		t.Fatalf("Send with State failed: %v", err)
	}

	// Verify state updated (mock responder should trigger state update logic in Send)
	state, _ := sm.Load()
	if state.TokenUsage.TotalTokens == 0 {
		t.Error("Expected token usage to be updated")
	}
}

func TestOpenRouterClient_GetConfig_CI(t *testing.T) {
	// Save original env
	origCI := os.Getenv("CI")
	origRecacCI := os.Getenv("RECAC_CI_MODE")
	defer func() {
		os.Setenv("CI", origCI)
		os.Setenv("RECAC_CI_MODE", origRecacCI)
	}()

	t.Run("Default MaxTokens", func(t *testing.T) {
		os.Unsetenv("CI")
		os.Unsetenv("RECAC_CI_MODE")
		client := NewOpenRouterClient("key", "model", "proj")
		config := client.getConfig()
		assert.Equal(t, 2048, config.MaxTokens)
	})

	t.Run("CI MaxTokens", func(t *testing.T) {
		os.Setenv("CI", "true")
		client := NewOpenRouterClient("key", "model", "proj")
		config := client.getConfig()
		assert.Equal(t, 1000, config.MaxTokens)
	})

	t.Run("RECAC_CI_MODE MaxTokens", func(t *testing.T) {
		os.Unsetenv("CI")
		os.Setenv("RECAC_CI_MODE", "true")
		client := NewOpenRouterClient("key", "model", "proj")
		config := client.getConfig()
		assert.Equal(t, 1000, config.MaxTokens)
	})
}
