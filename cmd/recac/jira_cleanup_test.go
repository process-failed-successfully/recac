package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/jira"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
)

// MockRoundTripper allows mocking HTTP responses
type MockRoundTripper struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req)
}

func TestJiraCleanupCmd_ConfirmYes(t *testing.T) {
	// Mock exit
	originalExit := exit
	defer func() { exit = originalExit }()
	exit = func(code int) {
		panic(fmt.Sprintf("exit called with code %d", code))
	}

	// Mock Jira Client
	originalFactory := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalFactory }()

	mockRT := &MockRoundTripper{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			// Mock Search (LoadLabelIssues)
			if req.Method == "GET" && req.URL.Path == "/rest/api/3/search/jql" {
				resp := struct {
					Issues []map[string]interface{} `json:"issues"`
				}{
					Issues: []map[string]interface{}{
						{"key": "PROJ-1"},
						{"key": "PROJ-2"},
					},
				}
				data, _ := json.Marshal(resp)
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(data)),
					Header:     make(http.Header),
				}, nil
			}
			// Mock Delete
			if req.Method == "DELETE" {
				return &http.Response{
					StatusCode: 204,
					Body:       io.NopCloser(bytes.NewReader(nil)),
				}, nil
			}
			return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewReader(nil))}, nil
		},
	}

	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		client := jira.NewClient("https://mock.jira", "user", "token")
		client.HTTPClient.Transport = mockRT
		return client, nil
	}

	// Mock Survey
	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		// Verify prompt message
		_, ok := p.(*survey.Confirm)
		if ok {
			// Simulate "Yes"
			*(response.(*bool)) = true
			return nil
		}
		return fmt.Errorf("unexpected prompt type")
	}

	// Run Command
	cmd := jiraCleanupCmd
	// Reset flags
	cmd.Flags().Set("label", "test-label")
	cmd.Flags().Set("force", "false")

	// Capture output to avoid pollution
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Code panicked: %v", r)
		}
	}()

	cmd.Run(cmd, []string{})
}

func TestJiraCleanupCmd_ConfirmNo(t *testing.T) {
	// Mock exit
	originalExit := exit
	defer func() { exit = originalExit }()
	exit = func(code int) {
		panic(fmt.Sprintf("exit called with code %d", code))
	}

	// Mock Jira Client
	originalFactory := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalFactory }()

	deletedCount := 0
	mockRT := &MockRoundTripper{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			if req.Method == "GET" {
				resp := struct {
					Issues []map[string]interface{} `json:"issues"`
				}{
					Issues: []map[string]interface{}{{"key": "PROJ-1"}},
				}
				data, _ := json.Marshal(resp)
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(data)),
					Header:     make(http.Header),
				}, nil
			}
			if req.Method == "DELETE" {
				deletedCount++
				return &http.Response{StatusCode: 204, Body: io.NopCloser(bytes.NewReader(nil))}, nil
			}
			return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewReader(nil))}, nil
		},
	}

	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		client := jira.NewClient("https://mock.jira", "user", "token")
		client.HTTPClient.Transport = mockRT
		return client, nil
	}

	// Mock Survey - NO
	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		*(response.(*bool)) = false
		return nil
	}

	cmd := jiraCleanupCmd
    // Reset flags
    cmd.Flags().Set("label", "test-label")
    cmd.Flags().Set("force", "false")

	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Code panicked: %v", r)
		}
	}()

	cmd.Run(cmd, []string{})

	assert.Equal(t, 0, deletedCount, "Should not delete if not confirmed")
}

func TestJiraCleanupCmd_Force(t *testing.T) {
	// Mock exit
	originalExit := exit
	defer func() { exit = originalExit }()
	exit = func(code int) {
		panic(fmt.Sprintf("exit called with code %d", code))
	}

	// Mock Jira Client
	originalFactory := cmdutils.GetJiraClient
	defer func() { cmdutils.GetJiraClient = originalFactory }()

	deletedCount := 0
	mockRT := &MockRoundTripper{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			if req.Method == "GET" {
				resp := struct {
					Issues []map[string]interface{} `json:"issues"`
				}{
					Issues: []map[string]interface{}{{"key": "PROJ-1"}},
				}
				data, _ := json.Marshal(resp)
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(data)),
					Header:     make(http.Header),
				}, nil
			}
			if req.Method == "DELETE" {
				deletedCount++
				return &http.Response{StatusCode: 204, Body: io.NopCloser(bytes.NewReader(nil))}, nil
			}
			return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewReader(nil))}, nil
		},
	}

	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		client := jira.NewClient("https://mock.jira", "user", "token")
		client.HTTPClient.Transport = mockRT
		return client, nil
	}

	// Mock Survey - Should NOT be called
	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		t.Fatal("Prompt should not be called with --force")
		return nil
	}

	cmd := jiraCleanupCmd
    // Reset flags
    cmd.Flags().Set("label", "test-label")
    cmd.Flags().Set("force", "true")

	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Code panicked: %v", r)
		}
	}()

	cmd.Run(cmd, []string{})

	assert.Equal(t, 1, deletedCount, "Should delete with force")
}
