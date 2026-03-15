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
	"github.com/stretchr/testify/assert"
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

func TestRunProxyValidation(t *testing.T) {
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

	cmd := proxyCmd
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	t.Run("Missing target", func(t *testing.T) {
		proxyGenerate = false
		proxyTarget = ""
		err := runProxy(cmd, []string{})
		if err == nil || !strings.Contains(err.Error(), "--target is required") {
			t.Errorf("expected missing target error, got: %v", err)
		}
	})

	t.Run("Invalid target URL", func(t *testing.T) {
		proxyGenerate = false
		proxyTarget = "http://[fe80::1%en0]:8080/" // valid URL in go 1.19, maybe use a control char
		// A known bad URL parse:
		proxyTarget = "http://%42:8080/"
		err := runProxy(cmd, []string{})
		if err == nil || !strings.Contains(err.Error(), "invalid target URL") {
			t.Errorf("expected invalid target URL error, got: %v", err)
		}
	})
}

func TestSaveInteractionJSONLError(t *testing.T) {
	// Attempt to save to a directory to force an open file error
	invalidPath := t.TempDir()

	i := Interaction{
		Timestamp: time.Now(),
		Request:   ReqDump{Method: "GET"},
	}

	// This shouldn't panic
	saveInteractionJSONL(i, invalidPath)
}

func TestRecordingTransport_ReadBodyError(t *testing.T) {
	// Create an errReader
	req, _ := http.NewRequest("POST", "http://example.com", &errReader{})

	rt := &recordingTransport{
		transport: http.DefaultTransport, // Doesn't matter, we just need to hit the read body path
		onRecord: func(i Interaction) {},
	}

	// We only want to test the body reading part for coverage, we expect this request to fail in Transport
	rt.RoundTrip(req)
}

type errReader struct{}
func (e *errReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}
func (e *errReader) Close() error {
	return nil
}

func TestRecordingTransport_ResponseReadBodyError(t *testing.T) {
	// Inject a transport that returns a response with a bad body
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	rt := &recordingTransport{
		transport: &mockResponseTransport{},
		onRecord: func(i Interaction) {},
	}

	rt.RoundTrip(req)
}

type mockResponseTransport struct{}

func (m *mockResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(&errReader{}),
	}, nil
}

func TestSaveInteractionJSONL_SuccessCoverage(t *testing.T) {
	validPath := filepath.Join(t.TempDir(), "success.jsonl")

	i := Interaction{
		Timestamp: time.Now(),
		Request:   ReqDump{Method: "GET"},
	}

	saveInteractionJSONL(i, validPath)

	data, _ := os.ReadFile(validPath)
	if len(data) == 0 {
		t.Errorf("Expected data to be written")
	}
}

type errorTransport struct{}

func (e *errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("simulated transport error")
}

func TestRecordingTransportError(t *testing.T) {
	rt := &recordingTransport{
		transport: &errorTransport{},
		onRecord: func(i Interaction) {
			t.Errorf("Should not record on transport error")
		},
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)

	_, err := rt.RoundTrip(req)
	if err == nil || !strings.Contains(err.Error(), "simulated transport error") {
		t.Errorf("Expected simulated transport error, got %v", err)
	}
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

func TestRunProxyGenerationErrors(t *testing.T) {
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

	cmd := proxyCmd
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	t.Run("Missing record file", func(t *testing.T) {
		proxyGenerate = true
		proxyTarget = ""
		proxyRecordFile = filepath.Join(t.TempDir(), "non_existent.json")

		err := runProxy(cmd, []string{})
		if err == nil || !strings.Contains(err.Error(), "failed to read recording file") {
			t.Errorf("expected failed to read error, got: %v", err)
		}
	})

	t.Run("Empty recording", func(t *testing.T) {
		proxyGenerate = true
		proxyTarget = ""
		recordFile := filepath.Join(t.TempDir(), "empty.json")
		os.WriteFile(recordFile, []byte(""), 0644)
		proxyRecordFile = recordFile

		err := runProxy(cmd, []string{})
		if err == nil || !strings.Contains(err.Error(), "recording is empty") {
			t.Errorf("expected recording is empty error, got: %v", err)
		}
	})

	t.Run("Agent factory error", func(t *testing.T) {
		proxyGenerate = true
		proxyTarget = ""

		recordFile := filepath.Join(t.TempDir(), "rec.json")
		interactions := []Interaction{
			{Request: ReqDump{Method: "GET", URL: "/api/test"}},
		}
		data, _ := json.Marshal(interactions)
		os.WriteFile(recordFile, data, 0644)
		proxyRecordFile = recordFile

		origFactory := agentClientFactory
		defer func() { agentClientFactory = origFactory }()
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return nil, fmt.Errorf("factory error")
		}

		err := runProxy(cmd, []string{})
		if err == nil || !strings.Contains(err.Error(), "failed to create agent") {
			t.Errorf("expected failed to create agent error, got: %v", err)
		}
	})

	t.Run("Agent send error", func(t *testing.T) {
		proxyGenerate = true
		proxyTarget = ""

		recordFile := filepath.Join(t.TempDir(), "rec.json")
		interactions := []Interaction{
			{Request: ReqDump{Method: "GET", URL: "/api/test"}},
		}
		data, _ := json.Marshal(interactions)
		os.WriteFile(recordFile, data, 0644)
		proxyRecordFile = recordFile

		origFactory := agentClientFactory
		defer func() { agentClientFactory = origFactory }()
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &mockAgent{
				SendFunc: func(ctx context.Context, prompt string) (string, error) {
					return "", fmt.Errorf("send error")
				},
			}, nil
		}

		err := runProxy(cmd, []string{})
		if err == nil || !strings.Contains(err.Error(), "send error") {
			t.Errorf("expected send error, got: %v", err)
		}
	})

	t.Run("Write file error", func(t *testing.T) {
		proxyGenerate = true
		proxyTarget = ""

		recordFile := filepath.Join(t.TempDir(), "rec.json")
		interactions := []Interaction{
			{Request: ReqDump{Method: "GET", URL: "/api/test"}},
		}
		data, _ := json.Marshal(interactions)
		os.WriteFile(recordFile, data, 0644)
		proxyRecordFile = recordFile

		// Use a directory instead of a file to trigger a write error
		proxyOutput = t.TempDir()

		origFactory := agentClientFactory
		defer func() { agentClientFactory = origFactory }()
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &mockAgent{
				SendFunc: func(ctx context.Context, prompt string) (string, error) {
					return "```go\nfunc TestIntegration(t *testing.T) {}\n```", nil
				},
			}, nil
		}

		err := runProxy(cmd, []string{})
		if err == nil || !strings.Contains(err.Error(), "is a directory") {
			t.Errorf("expected write file error, got: %v", err)
		}
	})

	t.Run("Valid JSON but not JSONL lines", func(t *testing.T) {
		proxyGenerate = true
		proxyTarget = ""
		recordFile := filepath.Join(t.TempDir(), "rec.json")
		// Write a line that is valid JSON but not an interaction,
		// Unmarshal to Interaction won't fail if it's `{}` but we want to cover the fallback branch
		// Actually, if unmarshal fails, it continues. Let's write invalid json.
		os.WriteFile(recordFile, []byte("not_json\n"), 0644)
		proxyRecordFile = recordFile

		err := runProxy(cmd, []string{})
		if err == nil || !strings.Contains(err.Error(), "recording is empty") {
			t.Errorf("expected recording is empty error, got: %v", err)
		}
	})

	t.Run("Truncate interactions", func(t *testing.T) {
		proxyGenerate = true
		proxyTarget = ""
		recordFile := filepath.Join(t.TempDir(), "rec.json")
		var interactions []Interaction
		for i := 0; i < 15; i++ {
			interactions = append(interactions, Interaction{Request: ReqDump{Method: "GET", URL: "/api/test"}})
		}
		data, _ := json.Marshal(interactions)
		os.WriteFile(recordFile, data, 0644)
		proxyRecordFile = recordFile

		origFactory := agentClientFactory
		defer func() { agentClientFactory = origFactory }()
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &mockAgent{
				SendFunc: func(ctx context.Context, prompt string) (string, error) {
					return "```go\nfunc TestIntegration(t *testing.T) {}\n```", nil
				},
			}, nil
		}

		proxyOutput = filepath.Join(t.TempDir(), "out.go")

		err := runProxy(cmd, []string{})
		if err != nil {
			t.Errorf("expected success, got: %v", err)
		}
	})

	t.Run("JSON array legacy fallback", func(t *testing.T) {
		proxyGenerate = true
		proxyTarget = ""
		recordFile := filepath.Join(t.TempDir(), "rec.json")
		// This will fail jsonl parsing because it's multiple lines,
		// or json array parsing works
		interactions := []Interaction{
			{Request: ReqDump{Method: "GET", URL: "/api/test"}},
		}
		data, _ := json.Marshal(interactions)
		// Write standard JSON array (not JSONL)
		os.WriteFile(recordFile, data, 0644)
		proxyRecordFile = recordFile

		// Mock agent to prevent actual API call
		origFactory := agentClientFactory
		defer func() { agentClientFactory = origFactory }()
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &mockAgent{
				SendFunc: func(ctx context.Context, prompt string) (string, error) {
					return "```go\nfunc TestIntegration(t *testing.T) {}\n```", nil
				},
			}, nil
		}

		proxyOutput = filepath.Join(t.TempDir(), "out.go")

		err := runProxy(cmd, []string{})
		if err != nil {
			t.Errorf("expected success, got: %v", err)
		}
	})
}

func TestRunProxy_Errors(t *testing.T) {
	// target is required
	proxyTarget = ""
	proxyGenerate = false
	err := runProxy(proxyCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--target is required")

	// invalid target
	proxyTarget = "http://[::1]:namedport" // invalid port
	err = runProxy(proxyCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid target URL")

	// Generation without file
	proxyTarget = ""
	proxyGenerate = true
	proxyRecordFile = "nonexistent_file.jsonl"
	err = runProxy(proxyCmd, []string{})
	assert.Error(t, err) // since file does not exist
}
