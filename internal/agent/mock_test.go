package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent(t *testing.T) {
	agent := NewMockAgent()

	prompt := "This is a test prompt that is long enough to be truncated"
	response, err := agent.Send(context.Background(), prompt)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !strings.Contains(response, "Mock agent response") {
		t.Errorf("Response missing prefix, got: %s", response)
	}

	if !strings.Contains(response, "I received your prompt") {
		t.Errorf("Response missing body, got: %s", response)
	}
}

func TestTruncateString(t *testing.T) {
	s := "hello world"
	if truncateString(s, 5) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", truncateString(s, 5))
	}
	if truncateString(s, 20) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", truncateString(s, 20))
	}
}

func TestMockAgent_SetResponse(t *testing.T) {
	agent := NewMockAgent()
	agent.SetResponse("custom response")

	resp, err := agent.Send(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if resp != "custom response" {
		t.Errorf("Expected 'custom response', got '%s'", resp)
	}
}

func TestMockAgent_SendStream(t *testing.T) {
	agent := NewMockAgent()
	agent.SetResponse("stream response")

	chunks := []string{}
	resp, err := agent.SendStream(context.Background(), "hello", func(chunk string) {
		chunks = append(chunks, chunk)
	})

	if err != nil {
		t.Fatalf("SendStream failed: %v", err)
	}
	if resp != "stream response" {
		t.Errorf("Expected 'stream response', got '%s'", resp)
	}
	if len(chunks) != 1 || chunks[0] != "stream response" {
		t.Errorf("Expected chunk 'stream response', got %v", chunks)
	}
}

func TestMockAgent_SmokeTests(t *testing.T) {
	agent := NewMockAgent()

	// Test Prime Python spec
	resp, err := agent.Send(context.Background(), "ID:[PRIMES] Prime Number Script")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "Create Prime Number Script") {
		t.Errorf("Expected prime script plan, got '%s'", resp)
	}

	// Test python script implementation
	resp, err = agent.Send(context.Background(), "Create a python script named 'primes.py'")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "I will implement the prime number script") {
		t.Errorf("Expected prime script implementation, got '%s'", resp)
	}

	// Test guided tour
	resp, err = agent.Send(context.Background(), "Create a guided tour of this codebase")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !strings.Contains(resp, "Project Overview") {
		t.Errorf("Expected guided tour, got '%s'", resp)
	}
}

func TestMockAgent_SetError(t *testing.T) {
	agent := NewMockAgent()
	expectedErr := errors.New("mock error")
	agent.SetError(expectedErr)

	resp, err := agent.Send(context.Background(), "test prompt")
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, "", resp)
}
