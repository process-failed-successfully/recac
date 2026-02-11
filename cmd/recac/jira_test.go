package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/jira"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestJiraGetCmd(t *testing.T) {
	// Mock Jira Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/PROJ-123" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"key": "PROJ-123", "fields": {"summary": "Test Ticket"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Mock GetJiraClient
	originalGetJiraClient := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(server.URL, "user", "token"), nil
	}

	// Capture Stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute Command
	rootCmd.SetArgs([]string{"jira", "get", "--id", "PROJ-123"})
	rootCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	// Read Output
	var buf [1024]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	assert.Contains(t, output, "Ticket: PROJ-123")
	assert.Contains(t, output, "Title: Test Ticket")
}

func TestJiraTransitionCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/PROJ-123/transitions" {
			if r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"transitions": [{"id": "31", "name": "In Progress"}]}`))
				return
			}
			if r.Method == "POST" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalGetJiraClient := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(server.URL, "user", "token"), nil
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"jira", "transition", "--id", "PROJ-123", "--transition", "In Progress"})
	rootCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf [1024]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	assert.Contains(t, output, "Success: Ticket PROJ-123 transitioned to 'In Progress'")
}

func TestJiraCleanupCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"issues": [{"key": "PROJ-1"}]}`))
			return
		}
		if r.Method == "DELETE" && r.URL.Path == "/rest/api/3/issue/PROJ-1" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalGetJiraClient := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(server.URL, "user", "token"), nil
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"jira", "cleanup", "--label", "test-label"})
	rootCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf [1024]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	assert.Contains(t, output, "Deleting PROJ-1... Success")
}

// MockAgent for testing
type mockAgentForJira struct {
	agent.Agent
	response string
}

func (m *mockAgentForJira) Send(ctx context.Context, prompt string) (string, error) {
	return m.response, nil
}

func (m *mockAgentForJira) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(m.response)
	}
	return m.response, nil
}

func TestJiraGenerateFromSpecCmd(t *testing.T) {
	// 1. Mock Jira Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock Create Ticket
		if r.Method == "POST" && r.URL.Path == "/rest/api/3/issue" {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"key": "PROJ-101"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalGetJiraClient := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(server.URL, "user", "token"), nil
	}

	// 2. Mock Agent Factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &mockAgentForJira{
			response: `[{"title": "Epic 1", "description": "Desc 1", "type": "Epic", "children": []}]`,
		}, nil
	}

	// 3. Create dummy spec file
	specPath := "test_spec.txt"
	os.WriteFile(specPath, []byte("Test Spec"), 0644)
	defer os.Remove(specPath)

	// 4. Config
	viper.Set("jira.project_key", "PROJ")
	defer viper.Set("jira.project_key", "")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"jira", "generate-from-spec", "--spec", specPath, "--project", "PROJ", "--repo-url", "http://repo.git"})
	rootCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf [2048]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	assert.Contains(t, output, "Created Epic PROJ-101")
}

func TestJiraGenerateFromArchCmd(t *testing.T) {
	// 1. Mock Jira Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/rest/api/3/issue" {
			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)
			fields := payload["fields"].(map[string]interface{})
			summary := fields["summary"].(string)

			// Simple counter logic simulation or just random
			key := "PROJ-999"
			if strings.Contains(summary, "System") {
				key = "PROJ-100"
			}
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(fmt.Sprintf(`{"key": "%s"}`, key)))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalGetJiraClient := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(server.URL, "user", "token"), nil
	}

	// 2. Create dummy arch file
	archPath := "test_arch.yaml"
	archContent := `
system_name: "Test System"
components:
  - id: "COMP1"
    description: "Component 1"
    type: "Service"
    implementation_steps: ["Step 1"]
`
	os.WriteFile(archPath, []byte(archContent), 0644)
	defer os.Remove(archPath)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"jira", "generate-from-arch", "--arch", archPath, "--project", "PROJ", "--repo-url", "http://repo.git"})
	rootCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf [2048]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	assert.Contains(t, output, "Created Epic PROJ-100") // System epic
}
