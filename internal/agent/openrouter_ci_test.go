package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestOpenRouterClient_CI_MaxTokens(t *testing.T) {
	// Enable CI mode
	os.Setenv("RECAC_CI_MODE", "true")
	defer os.Unsetenv("RECAC_CI_MODE")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqMap map[string]interface{}
		if err := json.Unmarshal(body, &reqMap); err != nil {
			t.Errorf("Failed to unmarshal request body: %v", err)
		}

		// Verify max_tokens is set
		if mt, ok := reqMap["max_tokens"]; ok {
			if mtFloat, ok := mt.(float64); ok {
				// Should be 900 / 2 = 450
				if mtFloat != 450 {
					t.Errorf("Expected max_tokens 450, got %v", mtFloat)
				}
			} else {
				t.Errorf("max_tokens is not a number")
			}
		} else {
			t.Errorf("max_tokens missing in request")
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"choices": [{"message": {"content": "OK"}}]}`)
	}))
	defer server.Close()

	client := NewOpenRouterClient("key", "model", "proj")
	client.apiURL = server.URL

	// Verify BaseClient.DefaultMaxTokens
	if client.DefaultMaxTokens != 900 {
		t.Errorf("Expected DefaultMaxTokens 900 in CI, got %d", client.DefaultMaxTokens)
	}

	_, err := client.Send(context.Background(), "test")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
}

func TestOpenRouterClient_NonCI_MaxTokens(t *testing.T) {
	// Ensure CI mode is off
	os.Unsetenv("RECAC_CI_MODE")
	os.Unsetenv("CI")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqMap map[string]interface{}
		json.Unmarshal(body, &reqMap)

		// Verify max_tokens is set (since 128000 > 0, it should be 64000)
		if mt, ok := reqMap["max_tokens"]; ok {
			if mtFloat, ok := mt.(float64); ok {
				if mtFloat != 64000 {
					t.Errorf("Expected max_tokens 64000, got %v", mtFloat)
				}
			}
		} else {
			t.Errorf("max_tokens missing in request (expected 64000)")
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"choices": [{"message": {"content": "OK"}}]}`)
	}))
	defer server.Close()

	client := NewOpenRouterClient("key", "model", "proj")
	client.apiURL = server.URL

	if client.DefaultMaxTokens != 128000 {
		t.Errorf("Expected DefaultMaxTokens 128000, got %d", client.DefaultMaxTokens)
	}

	client.Send(context.Background(), "test")
}

func TestOpenRouterClient_CI_Stream_MaxTokens(t *testing.T) {
	// Enable CI mode
	os.Setenv("RECAC_CI_MODE", "true")
	defer os.Unsetenv("RECAC_CI_MODE")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqMap map[string]interface{}
		json.Unmarshal(body, &reqMap)

		if mt, ok := reqMap["max_tokens"]; ok {
			if mtFloat, ok := mt.(float64); ok {
				if mtFloat != 450 {
					t.Errorf("Expected max_tokens 450, got %v", mtFloat)
				}
			}
		} else {
			t.Errorf("max_tokens missing in stream request")
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"choices\": [{\"delta\": {\"content\": \"OK\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewOpenRouterClient("key", "model", "proj")
	client.apiURL = server.URL

	_, err := client.SendStream(context.Background(), "test", func(s string) {})
	if err != nil {
		t.Fatalf("SendStream failed: %v", err)
	}
}
