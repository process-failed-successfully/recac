package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"
)

// CheatMockAgent implements agent.Agent interface
type CheatMockAgent struct {
	SendFunc       func(ctx context.Context, prompt string) (string, error)
	SendStreamFunc func(ctx context.Context, prompt string, onChunk func(string)) (string, error)
}

func (m *CheatMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "mock response", nil
}

func (m *CheatMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if m.SendStreamFunc != nil {
		return m.SendStreamFunc(ctx, prompt, onChunk)
	}
	onChunk("mock stream response")
	return "mock stream response", nil
}

func TestCheatCmd_Offline(t *testing.T) {
	// Restore original factory and flags
	defer func() {
		cheatNoOnline = false
	}()

	cheatNoOnline = true // Force offline

	cmd := cheatCmd
	b := bytes.NewBufferString("")
	cmd.SetOut(b)

	// Test embedded
	cmd.SetArgs([]string{"git"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	output := b.String()
	if !strings.Contains(output, "Embedded Cheat Sheet for 'git'") {
		t.Errorf("expected embedded cheat sheet, got: %s", output)
	}
}

func TestCheatCmd_Online(t *testing.T) {
	// Mock cht.sh
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "unknown") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, "# Mock Cheat Sheet")
	}))
	defer ts.Close()

	// Override URL
	oldURL := cheatShURL
	cheatShURL = ts.URL
	defer func() { cheatShURL = oldURL }()

	cmd := cheatCmd
	b := bytes.NewBufferString("")
	cmd.SetOut(b)

	// Test online
	cmd.SetArgs([]string{"curl"}) // 'curl' is not in embedded map
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	output := b.String()
	if !strings.Contains(output, "Searching cht.sh") {
		t.Error("expected searching message")
	}
	if !strings.Contains(output, "# Mock Cheat Sheet") {
		t.Errorf("expected mock content, got: %s", output)
	}
}

func TestCheatCmd_AIFallback(t *testing.T) {
	// Mock cht.sh to fail
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	oldURL := cheatShURL
	cheatShURL = ts.URL
	defer func() { cheatShURL = oldURL }()

	// Mock Agent
	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &CheatMockAgent{
			SendStreamFunc: func(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
				onChunk("# AI Generated Cheat Sheet")
				return "", nil
			},
		}, nil
	}
	defer func() { agentClientFactory = oldFactory }()

	cmd := cheatCmd
	b := bytes.NewBufferString("")
	cmd.SetOut(b)

	cmd.SetArgs([]string{"obscure-tool"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	output := b.String()
	if !strings.Contains(output, "Asking AI") {
		t.Error("expected AI fallback message")
	}
	if !strings.Contains(output, "# AI Generated Cheat Sheet") {
		t.Errorf("expected AI content, got: %s", output)
	}
}

func TestCheatCmd_Suggestions(t *testing.T) {
	// Create temp dir with Makefile
	tmpDir, err := os.MkdirTemp("", "cheat-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte("target:"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change cwd
	oldCwd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldCwd)

	cmd := cheatCmd
	b := bytes.NewBufferString("")
	cmd.SetOut(b)

	// Test suggestions flag
	cheatSuggest = true
	defer func() { cheatSuggest = false }()

	cmd.SetArgs([]string{}) // No topic
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	output := b.String()
	if !strings.Contains(output, "Found Makefile") {
		t.Errorf("expected makefile suggestion, got: %s", output)
	}
}
