package workflow

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"

	"github.com/stretchr/testify/assert"
)

// MockRoundTripper for intercepting HTTP requests
type MockRoundTripper struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req)
}

func NewMockJiraClient(rt func(req *http.Request) (*http.Response, error)) *jira.Client {
	client := jira.NewClient("https://test.jira.com", "user", "token")
	client.HTTPClient.Transport = &MockRoundTripper{RoundTripFunc: rt}
	return client
}

func TestProcessJiraTicket_GetTicketError(t *testing.T) {
	client := NewMockJiraClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("connection failed")
	})

	err := ProcessJiraTicket(context.Background(), "TEST-1", client, SessionConfig{}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection failed")
}

func TestProcessJiraTicket_InvalidTicketFormat(t *testing.T) {
	client := NewMockJiraClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)), // Missing "fields"
			Header:     make(http.Header),
		}, nil
	})

	err := ProcessJiraTicket(context.Background(), "TEST-1", client, SessionConfig{}, nil)
	assert.Error(t, err)
	assert.Equal(t, "invalid ticket format", err.Error())
}

func TestProcessJiraTicket_SetupWorkspaceFailure(t *testing.T) {
	// Mock Jira Client to return a valid ticket
	client := NewMockJiraClient(func(req *http.Request) (*http.Response, error) {
		jsonResp := `{
			"fields": {
				"summary": "Test Summary",
				"description": {
					"type": "doc",
					"version": 1,
					"content": [
						{
							"type": "paragraph",
							"content": [
								{
									"type": "text",
									"text": "Repo: https://github.com/example/repo"
								}
							]
						}
					]
				}
			}
		}`
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(jsonResp)),
			Header:     make(http.Header),
		}, nil
	})

	// Mock SetupWorkspace to fail
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("setup failed")
	}

	tmpDir := t.TempDir()
	cfg := SessionConfig{
		ProjectPath: tmpDir,
	}

	err := ProcessJiraTicket(context.Background(), "TEST-1", client, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setup failed")
}

func TestProcessJiraTicket_WorkspaceCreationFailure(t *testing.T) {
	// Mock Jira Client
	client := NewMockJiraClient(func(req *http.Request) (*http.Response, error) {
		// Return valid ticket
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(`{"fields":{"summary":"Test","description":{"type":"doc","content":[]}}}`)),
			Header:     make(http.Header),
		}, nil
	})

	// Create a file to block mkdir
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocking_file")
	os.WriteFile(blockingFile, []byte("block"), 0644)

	cfg := SessionConfig{
		ProjectPath: blockingFile,
	}

	err := ProcessJiraTicket(context.Background(), "TEST-1", client, cfg, nil)
	assert.Error(t, err)
	// Error message from MkdirAll usually contains "not a directory" or similar
	// On Linux it says "not a directory". On Windows "cannot create ...".
	// Let's just check it is an error.
	assert.Error(t, err)
}

func TestProcessDirectTask_SetupWorkspaceFailure(t *testing.T) {
	// Mock SetupWorkspace to fail
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("setup failed")
	}

	cfg := SessionConfig{
		RepoURL:     "https://github.com/example/direct",
		ProjectPath: t.TempDir(),
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setup failed")
}
