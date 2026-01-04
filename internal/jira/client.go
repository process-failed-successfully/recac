package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// JiraClient handles communication with Jira API
type JiraClient struct {
	BaseURL    string
	Username   string
	APIToken   string
	HTTPClient *http.Client
}

// Issue represents a Jira issue
type Issue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary   string `json:"summary"`
		Status    string `json:"status"`
		Assignee  string `json:"assignee"`
		Created   string `json:"created"`
		Updated   string `json:"updated"`
	} `json:"fields"`
}

// NewClient creates a new Jira client
func NewClient() (*JiraClient, error) {
	config, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	return &JiraClient{
		BaseURL:    config.JiraURL,
		Username:   config.JiraUsername,
		APIToken:   config.JiraAPIToken,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// GetIssue fetches a Jira issue by ID
func (c *JiraClient) GetIssue(issueID string) (*Issue, error) {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s", c.BaseURL, issueID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var issue Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return &issue, nil
}

// SearchIssues searches for Jira issues using JQL
func (c *JiraClient) SearchIssues(jql string) ([]Issue, error) {
	url := fmt.Sprintf("%s/rest/api/2/search", c.BaseURL)

	payload := map[string]interface{}{
		"jql":        jql,
		"maxResults": 50,
		"fields":     []string{"summary", "status", "assignee", "created", "updated"},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Issues []Issue `json:"issues"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return result.Issues, nil
}

// UpdateIssueStatus updates the status of a Jira issue
func (c *JiraClient) UpdateIssueStatus(issueID, status string) error {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.BaseURL, issueID)

	payload := map[string]interface{}{
		"transition": map[string]interface{}{
			"id": status,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
