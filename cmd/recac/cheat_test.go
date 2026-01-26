package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type CheatMockAgent struct {
	Response string
}

func (m *CheatMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *CheatMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestCheatCmd(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	// Mock cht.sh server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		topic := strings.TrimPrefix(r.URL.Path, "/")
		if topic == "python" {
			fmt.Fprint(w, "# Python Cheatsheet\nprint('Hello')")
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Save and restore globals
	origClient := cheatHTTPClient
	origURL := cheatShURL
	origFactory := agentClientFactory
	defer func() {
		cheatHTTPClient = origClient
		cheatShURL = origURL
		agentClientFactory = origFactory
	}()

	cheatHTTPClient = server.Client()
	cheatShURL = server.URL

	// Mock Agent Factory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &CheatMockAgent{Response: "# AI Cheatsheet\nAI generated content"}, nil
	}

	t.Run("Static Cheatsheet", func(t *testing.T) {
		cmd := cheatCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		// Run with 'tar' which is in static list
		err := runCheat(cmd, []string{"tar"})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "tar -czvf")
	})

	t.Run("Cht.sh Fetch", func(t *testing.T) {
		cmd := cheatCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		// Run with 'python' which is mocked in httptest server
		err := runCheat(cmd, []string{"python"})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Fetching from cht.sh/python")
		assert.Contains(t, buf.String(), "print('Hello')")
	})

	t.Run("AI Fallback", func(t *testing.T) {
		cmd := cheatCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer)) // Capture stderr too

		// Run with 'unknown_topic' which should fail cht.sh and go to AI
		err := runCheat(cmd, []string{"unknown_topic"})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Asking AI for a cheatsheet")
		assert.Contains(t, buf.String(), "AI generated content")
	})

	t.Run("Context Detection", func(t *testing.T) {
		// Create a go.mod file
		if err := os.WriteFile("go.mod", []byte("module test"), 0644); err != nil {
			t.Fatal(err)
		}
		defer os.Remove("go.mod")

		cmd := cheatCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		// Run without args
		err := runCheat(cmd, []string{})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Detected context: go")
	})

	t.Run("No Context No Args", func(t *testing.T) {
		// Ensure empty dir
		os.Remove("go.mod")

		cmd := cheatCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := runCheat(cmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no topic provided and could not detect context")
	})
}
