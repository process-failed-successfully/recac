package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"recac/internal/cmdutils"
	"recac/internal/jira"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTransport struct {
	Response *http.Response
	Err      error
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Response, nil
}

func TestJiraListCmd(t *testing.T) {
	// Mock GetJiraClient
	originalGetJiraClient := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()

	mockResponseData := map[string]interface{}{
		"issues": []map[string]interface{}{
			{
				"key": "PROJ-1",
				"fields": map[string]interface{}{
					"summary": "First Ticket",
					"status": map[string]interface{}{
						"name": "To Do",
					},
					"issuetype": map[string]interface{}{
						"name": "Story",
					},
				},
			},
			{
				"key": "PROJ-2",
				"fields": map[string]interface{}{
					"summary": "Second Ticket",
					"status": map[string]interface{}{
						"name": "In Progress",
					},
					"issuetype": map[string]interface{}{
						"name": "Bug",
					},
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(mockResponseData)

	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		client := jira.NewClient("http://mock", "user", "token")
		client.HTTPClient = &http.Client{
			Transport: &mockTransport{
				Response: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
					Header:     make(http.Header),
				},
			},
		}
		return client, nil
	}

	output, err := executeCommand(rootCmd, "jira", "list")
	require.NoError(t, err)

	assert.Contains(t, output, "PROJ-1")
	assert.Contains(t, output, "Story")
	assert.Contains(t, output, "To Do")
	assert.Contains(t, output, "First Ticket")

	assert.Contains(t, output, "PROJ-2")
	assert.Contains(t, output, "Bug")
	assert.Contains(t, output, "In Progress")
	assert.Contains(t, output, "Second Ticket")
}

func TestJiraListCmd_NoTickets(t *testing.T) {
	originalGetJiraClient := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()

	mockResponseData := map[string]interface{}{
		"issues": []interface{}{},
	}
	bodyBytes, _ := json.Marshal(mockResponseData)

	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		client := jira.NewClient("http://mock", "user", "token")
		client.HTTPClient = &http.Client{
			Transport: &mockTransport{
				Response: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
					Header:     make(http.Header),
				},
			},
		}
		return client, nil
	}

	output, err := executeCommand(rootCmd, "jira", "list")
	require.NoError(t, err)

	assert.Contains(t, output, "No tickets found.")
}

func TestJiraListCmd_MissingFields(t *testing.T) {
	originalGetJiraClient := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()

	mockResponseData := map[string]interface{}{
		"issues": []map[string]interface{}{
			{
				"key": "PROJ-3",
				"fields": map[string]interface{}{
					"summary": "Incomplete Ticket",
					// Missing status and issuetype
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(mockResponseData)

	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		client := jira.NewClient("http://mock", "user", "token")
		client.HTTPClient = &http.Client{
			Transport: &mockTransport{
				Response: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
					Header:     make(http.Header),
				},
			},
		}
		return client, nil
	}

	output, err := executeCommand(rootCmd, "jira", "list")
	require.NoError(t, err)

	assert.Contains(t, output, "PROJ-3")
	assert.Contains(t, output, "Incomplete Ticket")
	// Columns might be empty string/whitespace, so we just check it didn't panic and printed the key/summary
}
