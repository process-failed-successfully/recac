package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// AsanaPoller implements the Poller interface for Asana tasks.
type AsanaPoller struct {
	BaseURL string
	Token   string
	Project string
	Client  *http.Client
}

// NewAsanaPoller creates a new AsanaPoller.
func NewAsanaPoller(token, project string) *AsanaPoller {
	return &AsanaPoller{
		BaseURL: "https://app.asana.com/api/1.0",
		Token:   token,
		Project: project,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Ensure AsanaPoller implements Poller
var _ Poller = (*AsanaPoller)(nil)

func (p *AsanaPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	// Fetch incomplete tasks from the project
	url := fmt.Sprintf("%s/projects/%s/tasks?opt_fields=name,notes,completed", p.BaseURL, p.Project)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch asana tasks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("asana api error: %d %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID        string `json:"gid"`
			Name      string `json:"name"`
			Notes     string `json:"notes"`
			Completed bool   `json:"completed"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode asana response: %w", err)
	}

	var items []WorkItem
	for _, task := range result.Data {
		if task.Completed {
			continue // Skip completed tasks just in case API returned them
		}

		repoURL := extractRepoURL(task.Notes, RepoRegex)

		item := WorkItem{
			ID:          task.ID,
			Summary:     task.Name,
			Description: task.Notes,
			RepoURL:     repoURL,
			EnvVars: map[string]string{
				"ASANA_TASK_ID": task.ID,
			},
		}

		items = append(items, item)
	}

	return items, nil
}

func (p *AsanaPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	// 1. Post Comment
	if comment != "" {
		if err := p.postComment(ctx, item.ID, comment); err != nil {
			return err
		}
	}

	// 2. Complete task if Done
	if strings.EqualFold(status, "Done") || strings.EqualFold(status, "Closed") {
		return p.completeTask(ctx, item.ID)
	}

	return nil
}

func (p *AsanaPoller) postComment(ctx context.Context, taskID, text string) error {
	url := fmt.Sprintf("%s/tasks/%s/stories", p.BaseURL, taskID)

	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"text": text,
		},
	}
	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonPayload)))
	if err != nil {
		return err
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post asana comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to post asana comment: %d %s", resp.StatusCode, string(body))
	}
	return nil
}

func (p *AsanaPoller) completeTask(ctx context.Context, taskID string) error {
	url := fmt.Sprintf("%s/tasks/%s", p.BaseURL, taskID)

	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"completed": true,
		},
	}
	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(string(jsonPayload)))
	if err != nil {
		return err
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to complete asana task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to complete asana task: %d %s", resp.StatusCode, string(body))
	}
	return nil
}

func (p *AsanaPoller) Ping(ctx context.Context) error {
	// Simple query to verify connectivity and credentials
	// We'll just fetch the project
	url := fmt.Sprintf("%s/projects/%s", p.BaseURL, p.Project)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("asana connectivity check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("asana ping failed with status: %d", resp.StatusCode)
	}
	return nil
}

func (p *AsanaPoller) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
}
