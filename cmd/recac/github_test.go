package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/github"
	"testing"
)

// GHTestAgent implements agent.Agent for testing to avoid name collision
type GHTestAgent struct {
	Response string
}

func (m *GHTestAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *GHTestAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}

func TestGithubGenerateFromSpec(t *testing.T) {
	// 1. Mock HTTP Server for GitHub API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/repos/owner/repo/issues" {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"number": 1, "html_url": "http://github.com/owner/repo/issues/1"}`))
			return
		}
		if r.Method == "POST" && r.URL.Path == "/repos/owner/repo/issues/1/comments" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		// Allow any other POST to issues (for child tickets)
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"number": 2, "html_url": "http://github.com/owner/repo/issues/2"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// 2. Mock GetGitHubClient
	origGitHubClientFactory := cmdutils.GetGitHubClient
	defer func() { cmdutils.GetGitHubClient = origGitHubClientFactory }()

	cmdutils.GetGitHubClient = func(ctx context.Context) (*github.Client, error) {
		client := github.NewClient("token", "owner", "repo")
		client.BaseURL = server.URL
		return client, nil
	}

	// 3. Mock Agent
	origAgentFactory := agentClientFactory
	defer func() { agentClientFactory = origAgentFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &GHTestAgent{
			Response: `[{"title": "Epic", "type": "Epic", "children": [{"title": "Story", "type": "Story"}]}]`,
		}, nil
	}

	// 4. Create dummy spec file
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "app_spec.txt")
	os.WriteFile(specPath, []byte("Spec content"), 0644)

	// 5. Run Command
	cmd := githubGenerateFromSpecCmd
	// Reset flags manually
	cmd.Flags().Set("spec", specPath)
	cmd.Flags().Set("repo-url", "http://github.com/owner/repo")

	// Mock exit to prevent test from exiting
	origExit := exit
	defer func() { exit = origExit }()
	exit = func(code int) {
		if code != 0 {
			t.Fatalf("Exit called with code %d", code)
		}
	}

	runGithubGenerateFromSpec(cmd, []string{})
}
