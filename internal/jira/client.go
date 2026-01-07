package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client handles Jira API interactions.
type Client struct {
	BaseURL    string
	Username   string
	APIToken   string
	HTTPClient *http.Client
}

// NewClient creates a new Jira client.
func NewClient(baseURL, username, apiToken string) *Client {
	return &Client{
		BaseURL:  baseURL,
		Username: username,
		APIToken: apiToken,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Authenticate verifies the credentials by calling the Current User endpoint.
func (c *Client) Authenticate(ctx context.Context) error {
	url := fmt.Sprintf("%s/rest/api/3/myself", c.BaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed with status: %d", resp.StatusCode)
	}

	return nil
}

// GetTicket fetches a Jira ticket by ID.
func (c *Client) GetTicket(ctx context.Context, ticketID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.BaseURL, ticketID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch ticket with status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// TransitionIssue moves a ticket to a new status (e.g., "In Progress").
func (c *Client) TransitionIssue(ctx context.Context, ticketID, transitionID string) error {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.BaseURL, ticketID)

	payload := map[string]interface{}{
		"transition": map[string]string{
			"id": transitionID,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to transition issue with status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// AddComment adds a comment to a Jira ticket.
// The comment text is formatted in ADF (Atlassian Document Format) to preserve formatting.
func (c *Client) AddComment(ctx context.Context, ticketID, commentText string) error {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/comment", c.BaseURL, ticketID)

	// Format comment in ADF format to preserve formatting
	payload := map[string]interface{}{
		"body": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{
					"type": "paragraph",
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": commentText,
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to add comment with status: %d", resp.StatusCode)
	}

	return nil
}

// DeleteIssue deletes a Jira ticket.
func (c *Client) DeleteIssue(ctx context.Context, ticketID string) error {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.BaseURL, ticketID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete issue with status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// CreateTicket creates a new Jira ticket.
func (c *Client) CreateTicket(ctx context.Context, projectKey, summary, description, issueType string, labels []string) (string, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue", c.BaseURL)

	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"project": map[string]interface{}{
				"key": projectKey,
			},
			"summary": summary,
			"labels":  labels,
			"description": map[string]interface{}{
				"type":    "doc",
				"version": 1,
				"content": []map[string]interface{}{
					{
						"type": "paragraph",
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": description,
							},
						},
					},
				},
			},
			"issuetype": map[string]interface{}{
				"name": issueType,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create ticket with status: %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Key, nil
}

// ParseDescription extracts plain text from Jira's ADF (Atlassian Document Format).
func (c *Client) ParseDescription(data map[string]interface{}) string {
	fields, ok := data["fields"].(map[string]interface{})
	if !ok {
		return ""
	}

	description, ok := fields["description"].(map[string]interface{})
	if !ok {
		return ""
	}

	return extractTextFromADF(description)
}

func extractTextFromADF(node map[string]interface{}) string {
	var sb strings.Builder

	if text, ok := node["text"].(string); ok {
		sb.WriteString(text)
	}

	if content, ok := node["content"].([]interface{}); ok {
		for _, child := range content {
			if childMap, ok := child.(map[string]interface{}); ok {
				sb.WriteString(extractTextFromADF(childMap))
				// Add newlines for paragraphs
				if childMap["type"] == "paragraph" {
					sb.WriteString("\n")
				}
			}
		}
	}

	return sb.String()
}

// SearchIssues searches for Jira tickets using JQL.
func (c *Client) SearchIssues(ctx context.Context, jql string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/api/3/search/jql", c.BaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("jql", jql)
	q.Add("fields", "summary,description,status,labels,issuelinks,parent")
	req.URL.RawQuery = q.Encode()

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to search issues with status: %d", resp.StatusCode)
	}

	var result struct {
		Issues []map[string]interface{} `json:"issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Issues, nil
}

// LoadLabelIssues fetches issues with a specific label.
func (c *Client) LoadLabelIssues(ctx context.Context, label string) ([]map[string]interface{}, error) {
	jql := fmt.Sprintf("labels = \"%s\"", label)
	return c.SearchIssues(ctx, jql)
}

// GetTransitions fetches available transitions for a Jira ticket.
func (c *Client) GetTransitions(ctx context.Context, ticketID string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.BaseURL, ticketID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch transitions with status: %d", resp.StatusCode)
	}

	var result struct {
		Transitions []map[string]interface{} `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Transitions, nil
}

// SmartTransition attempts to transition an issue by ID or Name.
func (c *Client) SmartTransition(ctx context.Context, ticketID, targetNameOrID string) error {
	transitions, err := c.GetTransitions(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("failed to fetch transitions: %w", err)
	}

	var foundID string
	for _, t := range transitions {
		id, _ := t["id"].(string)
		name, _ := t["name"].(string)

		if id == targetNameOrID || strings.EqualFold(name, targetNameOrID) {
			foundID = id
			break
		}
	}

	if foundID == "" {
		return fmt.Errorf("no transition found matching '%s'", targetNameOrID)
	}

	return c.TransitionIssue(ctx, ticketID, foundID)
}

// GetBlockers returns a list of tickets that block the given ticket and are not "Done".
func (c *Client) GetBlockers(ticket map[string]interface{}) []string {
	fields, ok := ticket["fields"].(map[string]interface{})
	if !ok {
		return nil
	}

	links, ok := fields["issuelinks"].([]interface{})
	if !ok {
		return nil
	}

	var blockers []string
	for _, link := range links {
		linkMap, ok := link.(map[string]interface{})
		if !ok {
			continue
		}

		linkType, ok := linkMap["type"].(map[string]interface{})
		if !ok {
			continue
		}

		// Look for "is blocked by" relationship (inward)
		// Or any type where name is "Blocks" and inward is "is blocked by"
		inward, _ := linkType["inward"].(string)
		if strings.EqualFold(inward, "is blocked by") {
			inwardIssue, ok := linkMap["inwardIssue"].(map[string]interface{})
			if ok {
				key, _ := inwardIssue["key"].(string)
				fields, _ := inwardIssue["fields"].(map[string]interface{})
				if fields != nil {
					status, _ := fields["status"].(map[string]interface{})
					if status != nil {
						statusName, _ := status["name"].(string)
						// If status is not "Done" or equivalent, it's a blocker
						if !isDoneStatus(statusName) {
							blockers = append(blockers, fmt.Sprintf("%s (%s)", key, statusName))
						}
					}
				}
			}
		}
	}

	return blockers
}

// AddIssueLink creates a link between two Jira tickets (e.g., "Blocks").
func (c *Client) AddIssueLink(ctx context.Context, inwardKey, outwardKey, linkType string) error {
	url := fmt.Sprintf("%s/rest/api/3/issueLink", c.BaseURL)

	payload := map[string]interface{}{
		"type": map[string]interface{}{
			"name": linkType,
		},
		"inwardIssue": map[string]interface{}{
			"key": inwardKey,
		},
		"outwardIssue": map[string]interface{}{
			"key": outwardKey,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create issue link with status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SetParent sets the parent of an issue (e.g. for Subtasks or Epics).
func (c *Client) SetParent(ctx context.Context, issueKey, parentKey string) error {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.BaseURL, issueKey)

	// Start with "parent" field (standard for subtasks and next-gen epics)
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"parent": map[string]interface{}{
				"key": parentKey,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set parent with status: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

// AddLabel adds a label to an existing ticket.
func (c *Client) AddLabel(ctx context.Context, key, label string) error {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.BaseURL, key)
	payload := map[string]interface{}{
		"update": map[string]interface{}{
			"labels": []map[string]interface{}{
				{"add": label},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add label with status: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

func isDoneStatus(status string) bool {
	doneStatuses := []string{"Done", "Closed", "Resolved", "Finished", "Passed"}
	for _, s := range doneStatuses {
		if strings.EqualFold(s, status) {
			return true
		}
	}
	return false
}
