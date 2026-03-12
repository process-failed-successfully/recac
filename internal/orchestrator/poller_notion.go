package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// NotionPoller implements the Poller interface for Notion databases.
type NotionPoller struct {
	BaseURL    string
	Token      string
	DatabaseID string
	Label      string
	Client     *http.Client
}

// NewNotionPoller creates a new NotionPoller.
func NewNotionPoller(token, databaseID, label string) *NotionPoller {
	return &NotionPoller{
		BaseURL:    "https://api.notion.com/v1",
		Token:      token,
		DatabaseID: databaseID,
		Label:      label,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Ensure NotionPoller implements Poller
var _ Poller = (*NotionPoller)(nil)

func (p *NotionPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	url := fmt.Sprintf("%s/databases/%s/query", p.BaseURL, p.DatabaseID)

	var payload map[string]interface{}

	// Example structure for querying by label if provided
	// We'll query for pages that don't have "Status" = "Done".
	// The implementation can be basic first.
	// For now, let's keep it simple: filter for a specific tag/label.
	if p.Label != "" {
		payload = map[string]interface{}{
			"filter": map[string]interface{}{
				"and": []interface{}{
					map[string]interface{}{
						"property": "Tags",
						"multi_select": map[string]interface{}{
							"contains": p.Label,
						},
					},
					map[string]interface{}{
						"property": "Status",
						"status": map[string]interface{}{
							"does_not_equal": "Done",
						},
					},
				},
			},
		}
	} else {
		payload = map[string]interface{}{
			"filter": map[string]interface{}{
				"property": "Status",
				"status": map[string]interface{}{
					"does_not_equal": "Done",
				},
			},
		}
	}

	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch notion pages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("notion api error: %d %s", resp.StatusCode, string(body))
	}

	var result struct {
		Results []struct {
			ID         string                 `json:"id"`
			Properties map[string]interface{} `json:"properties"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode notion response: %w", err)
	}

	var items []WorkItem
	for _, page := range result.Results {
		title := extractNotionTitle(page.Properties, "Name")
		if title == "" {
			title = extractNotionTitle(page.Properties, "Title")
		}

		// Also extract a potential description and repo URL from the page properties.
		// For simplicity, we assume properties "Description" and "RepoURL" might exist as rich_text.
		desc := extractNotionRichText(page.Properties, "Description")
		repoURL := extractNotionRichText(page.Properties, "RepoURL")

		if repoURL == "" && desc != "" {
			repoURL = extractRepoURL(desc, RepoRegex)
		}

		item := WorkItem{
			ID:          page.ID,
			Summary:     title,
			Description: desc,
			RepoURL:     repoURL,
			EnvVars: map[string]string{
				"NOTION_PAGE_ID": page.ID,
			},
		}

		items = append(items, item)
	}

	return items, nil
}

func (p *NotionPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	// 1. If Done, update the status property
	if strings.EqualFold(status, "Done") || strings.EqualFold(status, "Closed") {
		if err := p.updatePageStatus(ctx, item.ID, "Done"); err != nil {
			return err
		}
	}

	// 2. Add comment if provided (Notion calls them comments)
	if comment != "" {
		if err := p.addComment(ctx, item.ID, comment); err != nil {
			return err
		}
	}

	return nil
}

func (p *NotionPoller) updatePageStatus(ctx context.Context, pageID, statusName string) error {
	url := fmt.Sprintf("%s/pages/%s", p.BaseURL, pageID)

	payload := map[string]interface{}{
		"properties": map[string]interface{}{
			"Status": map[string]interface{}{
				"status": map[string]interface{}{
					"name": statusName,
				},
			},
		},
	}

	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return err
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update notion page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update notion page: %d %s", resp.StatusCode, string(body))
	}

	return nil
}

func (p *NotionPoller) addComment(ctx context.Context, pageID, text string) error {
	url := fmt.Sprintf("%s/comments", p.BaseURL)

	// Max text block size is 2000 for Notion
	if len(text) > 2000 {
		text = text[:1997] + "..."
	}

	payload := map[string]interface{}{
		"parent": map[string]interface{}{
			"page_id": pageID,
		},
		"rich_text": []interface{}{
			map[string]interface{}{
				"text": map[string]interface{}{
					"content": text,
				},
			},
		},
	}

	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return err
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post notion comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to post notion comment: %d %s", resp.StatusCode, string(body))
	}

	return nil
}

func (p *NotionPoller) Ping(ctx context.Context) error {
	// Ping by retrieving the database
	url := fmt.Sprintf("%s/databases/%s", p.BaseURL, p.DatabaseID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("notion connectivity check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notion ping failed with status: %d", resp.StatusCode)
	}
	return nil
}

func (p *NotionPoller) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")
}

// Helper functions to parse Notion property values
func extractNotionTitle(props map[string]interface{}, key string) string {
	prop, ok := props[key].(map[string]interface{})
	if !ok {
		return ""
	}

	titleArr, ok := prop["title"].([]interface{})
	if !ok || len(titleArr) == 0 {
		return ""
	}

	firstTitle, ok := titleArr[0].(map[string]interface{})
	if !ok {
		return ""
	}

	plainText, _ := firstTitle["plain_text"].(string)
	return plainText
}

func extractNotionRichText(props map[string]interface{}, key string) string {
	prop, ok := props[key].(map[string]interface{})
	if !ok {
		return ""
	}

	richTextArr, ok := prop["rich_text"].([]interface{})
	if !ok || len(richTextArr) == 0 {
		return ""
	}

	firstText, ok := richTextArr[0].(map[string]interface{})
	if !ok {
		return ""
	}

	plainText, _ := firstText["plain_text"].(string)
	return plainText
}
