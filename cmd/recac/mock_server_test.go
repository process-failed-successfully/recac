package main

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockServerTestAgent struct {
	Response string
	Err      error
}

func (m *MockServerTestAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func (m *MockServerTestAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, m.Err
}

func TestMockServer_Handler(t *testing.T) {
	// Setup
	mockAgent := &MockServerTestAgent{
		Response: "STATUS: 201\nBODY:\n{\"id\": 123, \"status\": \"created\"}",
	}

	// Create handler with context
	handler := NewMockServerHandler(context.Background(), mockAgent, "API Context", 0)

	// Execute Request
	req := httptest.NewRequest("POST", "/users", strings.NewReader(`{"name": "Bob"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler(w, req)

	// Assert Response
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, 201, resp.StatusCode)
	assert.JSONEq(t, `{"id": 123, "status": "created"}`, string(body))
}

func TestMockServer_Handler_Fallback(t *testing.T) {
	// Test fallback when agent returns just JSON without headers
	mockAgent := &MockServerTestAgent{
		Response: `{"error": "not found"}`,
	}

	handler := NewMockServerHandler(context.Background(), mockAgent, "API Context", 0)

	req := httptest.NewRequest("GET", "/unknown", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, 200, resp.StatusCode) // Defaults to 200
	assert.JSONEq(t, `{"error": "not found"}`, string(body))
}
