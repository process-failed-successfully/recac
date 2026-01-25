package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"recac/internal/agent"
)

func TestProxyRecording(t *testing.T) {
	// 1. Create Mock Target Server
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/test" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message": "success"}`))
			return
		}
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer targetServer.Close()

	targetURL, _ := url.Parse(targetServer.URL)
	recordFile := filepath.Join(t.TempDir(), "recording.json")

	// 2. Create Proxy Handler using the exported NewProxyHandler
	var recorded []Interaction
	handler := NewProxyHandler(targetURL, func(i Interaction) {
		recorded = append(recorded, i)
	}, recordFile)

	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()

	// 3. Send Request to Proxy
	client := proxyServer.Client()
	resp, err := client.Post(proxyServer.URL+"/api/test", "application/json", strings.NewReader(`{"foo":"bar"}`))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// 4. Verify Response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "success") {
		t.Errorf("Expected success in body, got %s", body)
	}

	// 5. Verify Recording (In-Memory)
	if len(recorded) != 1 {
		t.Errorf("Expected 1 recorded interaction, got %d", len(recorded))
	} else {
		// URL might be full URL or relative path depending on implementation details
		// but ReqDump should capture it
		if !strings.Contains(recorded[0].Request.URL, "/api/test") {
			t.Errorf("Recorded URL mismatch: %s", recorded[0].Request.URL)
		}
		if recorded[0].Response.Status != 200 {
			t.Errorf("Recorded status expected 200, got %d", recorded[0].Response.Status)
		}
	}

	// 6. Verify Recording (File - JSONL)
	content, err := os.ReadFile(recordFile)
	if err != nil {
		t.Fatalf("Failed to read record file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 1 {
		t.Errorf("Expected 1 line in file, got %d. Content: %s", len(lines), string(content))
	}
	var fileI Interaction
	if err := json.Unmarshal([]byte(lines[0]), &fileI); err != nil {
		t.Fatalf("Failed to parse JSONL line: %v", err)
	}
}

// Local mock agent implementation
type mockAgent struct {
	SendFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *mockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "", nil
}

func (m *mockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	s, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(s)
	}
	return s, err
}

func TestProxyGeneration(t *testing.T) {
	// Setup Mock Agent
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockResponse := "```go\nfunc TestIntegration(t *testing.T) {}\n```"
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &mockAgent{
			SendFunc: func(ctx context.Context, prompt string) (string, error) {
				if strings.Contains(prompt, "Generate comprehensive integration tests") {
					return mockResponse, nil
				}
				return "", fmt.Errorf("unexpected prompt: %s", prompt)
			},
		}, nil
	}

	// Create a dummy recording file
	recordFile := filepath.Join(t.TempDir(), "recording.json")
	interactions := []Interaction{
		{
			Timestamp: time.Now(),
			Request:   ReqDump{Method: "GET", URL: "/api/test"},
			Response:  ResDump{Status: 200, Body: "{}"},
		},
	}
	data, _ := json.Marshal(interactions)
	os.WriteFile(recordFile, data, 0644)

	outputFile := filepath.Join(t.TempDir(), "output_test.go")

	// Setup Command Variables (global in package main)
	// We need to be careful with global state in tests.
	// Saving original values is good practice.
	origRecord := proxyRecordFile
	origOut := proxyOutput
	origGen := proxyGenerate
	origTarget := proxyTarget
	origLang := proxyLanguage

	defer func() {
		proxyRecordFile = origRecord
		proxyOutput = origOut
		proxyGenerate = origGen
		proxyTarget = origTarget
		proxyLanguage = origLang
	}()

	proxyRecordFile = recordFile
	proxyOutput = outputFile
	proxyGenerate = true
	proxyTarget = "" // trigger generation mode
	proxyLanguage = "go"

	// Run Generation via command runner
	// We create a dummy command because runProxy uses cmd.OutOrStdout
	cmd := proxyCmd
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := runProxy(cmd, []string{})
	if err != nil {
		t.Fatalf("runProxy failed: %v", err)
	}

	// Verify Output File
	outContent, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Output file not created: %v", err)
	}
	if !strings.Contains(string(outContent), "func TestIntegration") {
		t.Errorf("Output file content wrong: %s", outContent)
	}
}
