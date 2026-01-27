package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
)

type MockAgentForResearch struct {
	LastPrompt string
	Response   string
}

func (m *MockAgentForResearch) Send(ctx context.Context, prompt string) (string, error) {
	m.LastPrompt = prompt
	return m.Response, nil
}

func (m *MockAgentForResearch) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	m.LastPrompt = prompt
	onChunk(m.Response)
	return m.Response, nil
}

func TestResearch(t *testing.T) {
	// 1. Mock Server for Search and Content
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If request is to search (POST to root in our mock config)
		if r.Method == "POST" {
			r.ParseForm()
			q := r.Form.Get("q")
			if q == "golang" {
				// Return mock DDG HTML
				// Point href to this server
				// r.Host gives us the host:port of the server as seen by client
				href := fmt.Sprintf("http://%s/page1", r.Host)
				html := fmt.Sprintf(`<html><body>
                 <div class="result">
                    <a class="result__a" href="%s">Go Programming Language</a>
                 </div>
                 </body></html>`, href)
				w.Header().Set("Content-Type", "text/html")
				w.Write([]byte(html))
				return
			}
		}

		// If request is for page content
		if r.URL.Path == "/page1" {
			w.Write([]byte(`<html><body><h1>Go</h1><p>Go is an open source programming language.</p></body></html>`))
			return
		}

		http.NotFound(w, r)
	}))
	defer ts.Close()

	// 2. Override Global Vars
	origClient := researchHttpClient
	origBaseURL := researchBaseURL
	origFactory := agentClientFactory
	defer func() {
		researchHttpClient = origClient
		researchBaseURL = origBaseURL
		agentClientFactory = origFactory
	}()

	researchHttpClient = ts.Client()
	researchBaseURL = ts.URL

	mockAgent := &MockAgentForResearch{
		Response: "Based on the research, Go is an open source language.",
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// 3. Run Command
	// We use the global researchCmd but need to set output
	cmd := researchCmd
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	// Since we are calling runResearch directly, we don't need cmd.Execute() arguments parsing
	err := runResearch(cmd, []string{"golang"})

	// 4. Verify
	assert.NoError(t, err)
	output := outBuf.String()

	assert.Contains(t, output, "Searching the web for: golang")
	// Verify it found the link
	assert.Contains(t, output, "Reading [1/1]: Go Programming Language")
	// Verify it called agent
	assert.Contains(t, output, "Synthesizing answer...")
	assert.Contains(t, output, "Based on the research")

	// Verify agent received content
	assert.Contains(t, mockAgent.LastPrompt, "Go is an open source programming language")
}
