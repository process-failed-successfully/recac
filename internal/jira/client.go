package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
		return fmt.Errorf("failed to transition issue with status: %d", resp.StatusCode)
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