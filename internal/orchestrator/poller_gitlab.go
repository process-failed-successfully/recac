package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GitLabPoller implements the Poller interface for GitLab Issues.
type GitLabPoller struct {
	BaseURL string
	Token   string
	Project string
	Label   string
	Client  *http.Client
}

// NewGitLabPoller creates a new GitLabPoller.
func NewGitLabPoller(url, token, project, label string) *GitLabPoller {
	if url == "" {
		url = "https://gitlab.com"
	}
	// Ensure no trailing slash
	url = strings.TrimSuffix(url, "/")

	return &GitLabPoller{
		BaseURL: url,
		Token:   token,
		Project: project, // ID or URL-encoded path like "user%2Frepo"
		Label:   label,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Poll fetches open issues with the specified label.
func (p *GitLabPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%s/issues?state=opened&labels=%s", p.BaseURL, p.Project, p.Label)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gitlab api error: %d %s", resp.StatusCode, string(body))
	}

	var issues []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var items []WorkItem
	for _, issue := range issues {
		numberVal, _ := issue["iid"].(float64)
		number := int(numberVal)
		title, _ := issue["title"].(string)
		body, _ := issue["description"].(string)

		// Try to extract Repo URL from body
		repoURL := extractRepoURL(body, RepoRegex)
		if repoURL == "" {
			// Try to get web_url and construct the repo URL
			if webURL, ok := issue["web_url"].(string); ok {
				// e.g. https://gitlab.com/owner/repo/-/issues/1
				parts := strings.Split(webURL, "/-/issues")
				if len(parts) > 0 {
					repoURL = parts[0]
				}
			}
		}

		id := fmt.Sprintf("gl-%d", number)

		item := WorkItem{
			ID:          id,
			Summary:     title,
			Description: body,
			RepoURL:     repoURL,
			EnvVars: map[string]string{
				"GITLAB_ISSUE": strconv.Itoa(number),
			},
		}
		items = append(items, item)
	}

	return items, nil
}

// UpdateStatus posts a comment and optionally closes the issue.
func (p *GitLabPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	issueNumStr := strings.TrimPrefix(item.ID, "gl-")

	// 1. Post Comment
	if comment != "" {
		if err := p.postComment(ctx, issueNumStr, comment); err != nil {
			return err
		}
	}

	// 2. Close if Done
	if strings.EqualFold(status, "Done") || strings.EqualFold(status, "Closed") {
		return p.closeIssue(ctx, issueNumStr)
	}

	return nil
}

func (p *GitLabPoller) postComment(ctx context.Context, issueNum, body string) error {
	url := fmt.Sprintf("%s/api/v4/projects/%s/issues/%s/notes", p.BaseURL, p.Project, issueNum)

	payload := map[string]string{"body": body}
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to post comment: %d", resp.StatusCode)
	}
	return nil
}

func (p *GitLabPoller) Ping(ctx context.Context) error {
	// Verify project existence and token validity
	url := fmt.Sprintf("%s/api/v4/projects/%s", p.BaseURL, p.Project)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach gitlab: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gitlab ping failed: %d", resp.StatusCode)
	}
	return nil
}

func (p *GitLabPoller) closeIssue(ctx context.Context, issueNum string) error {
	url := fmt.Sprintf("%s/api/v4/projects/%s/issues/%s", p.BaseURL, p.Project, issueNum)

	payload := map[string]string{"state_event": "close"}
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to close issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to close issue: %d", resp.StatusCode)
	}
	return nil
}

func (p *GitLabPoller) setHeaders(req *http.Request) {
	req.Header.Set("PRIVATE-TOKEN", p.Token)
	req.Header.Set("Content-Type", "application/json")
}
