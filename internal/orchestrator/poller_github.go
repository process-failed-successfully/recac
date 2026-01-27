package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RepoRegex matches strings like "Repo: https://github.com/owner/repo".
var RepoRegex = regexp.MustCompile(`(?i)Repo: (https?://\S+)`)

// GitHubPoller implements the Poller interface for GitHub Issues.
type GitHubPoller struct {
	BaseURL string
	Token   string
	Owner   string
	Repo    string
	Label   string
	Client  *http.Client
}

// NewGitHubPoller creates a new GitHubPoller.
func NewGitHubPoller(token, owner, repo, label string) *GitHubPoller {
	return &GitHubPoller{
		BaseURL: "https://api.github.com",
		Token:   token,
		Owner:   owner,
		Repo:    repo,
		Label:   label,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Poll fetches open issues with the specified label.
func (p *GitHubPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues?state=open&labels=%s", p.BaseURL, p.Owner, p.Repo, p.Label)
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
		return nil, fmt.Errorf("github api error: %d %s", resp.StatusCode, string(body))
	}

	var issues []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var items []WorkItem
	for _, issue := range issues {
		// Skip pull requests (they are also returned by issues endpoint)
		if _, isPR := issue["pull_request"]; isPR {
			continue
		}

		numberVal, _ := issue["number"].(float64)
		number := int(numberVal)
		title, _ := issue["title"].(string)
		body, _ := issue["body"].(string)

		// Extract Repo URL from body or default to current repo
		repoURL := extractRepoURL(body, RepoRegex)
		if repoURL == "" {
			// Default to the repo where the issue is hosted
			repoURL = fmt.Sprintf("https://github.com/%s/%s", p.Owner, p.Repo)
		}

		id := fmt.Sprintf("gh-%d", number)

		item := WorkItem{
			ID:          id,
			Summary:     title,
			Description: body,
			RepoURL:     repoURL,
			EnvVars: map[string]string{
				"GITHUB_ISSUE": strconv.Itoa(number),
			},
		}
		items = append(items, item)
	}

	return items, nil
}

// UpdateStatus posts a comment and optionally closes the issue.
func (p *GitHubPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	issueNumStr := strings.TrimPrefix(item.ID, "gh-")

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

func (p *GitHubPoller) postComment(ctx context.Context, issueNum, body string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%s/comments", p.BaseURL, p.Owner, p.Repo, issueNum)

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

func (p *GitHubPoller) closeIssue(ctx context.Context, issueNum string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%s", p.BaseURL, p.Owner, p.Repo, issueNum)

	payload := map[string]string{"state": "closed"}
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(jsonBody))
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

func (p *GitHubPoller) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "token "+p.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "recac-orchestrator")
}
