package agent

import (
	"context"
	"strings"
	"testing"
)

func TestMockAgent_Send(t *testing.T) {
	agent := NewMockAgent()

	tests := []struct {
		name           string
		prompt         string
		expectedSubstr string
		expectedJSON   bool
	}{
		{
			name:           "Standard Prompt",
			prompt:         "Hello agent",
			expectedSubstr: "I received your prompt",
			expectedJSON:   false,
		},
		{
			name:           "Ticket Generation Prompt",
			prompt:         "You are an expert Technical Program Manager. Please generate tickets.",
			expectedSubstr: "MOCK-1",
			expectedJSON:   true,
		},
		{
			name:           "Long Prompt",
			prompt:         strings.Repeat("A", 200),
			expectedSubstr: "Prompt preview",
			expectedJSON:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := agent.Send(context.Background(), tt.prompt)
			if err != nil {
				t.Fatalf("Send failed: %v", err)
			}

			if !strings.Contains(resp, tt.expectedSubstr) {
				t.Errorf("Response does not contain expected substring %q. Got: %q", tt.expectedSubstr, resp)
			}

			if tt.expectedJSON {
				if !strings.HasPrefix(resp, "[") || !strings.HasSuffix(resp, "]") {
					t.Errorf("Expected JSON array response, got: %q", resp)
				}
			}
		})
	}
}
