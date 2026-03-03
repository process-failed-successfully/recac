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

// LinearPoller implements the Poller interface for Linear issues.
type LinearPoller struct {
	BaseURL string
	Token   string
	Team    string
	Label   string // Used to filter by a specific label name
	Client  *http.Client
}

// NewLinearPoller creates a new LinearPoller.
func NewLinearPoller(token, team, label string) *LinearPoller {
	return &LinearPoller{
		BaseURL: "https://api.linear.app/graphql",
		Token:   token,
		Team:    team,
		Label:   label,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Poll fetches issues from Linear for the configured team and label.
func (p *LinearPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	// Query to get issues for a team, optionally filtered by a label
	// Exclude issues in completed or canceled states
	var query string
	if p.Label != "" {
		query = `
		query {
			issues(
				filter: {
					team: { id: { eq: "` + p.Team + `" } },
					state: { type: { nin: ["completed", "canceled"] } },
					labels: { name: { eq: "` + p.Label + `" } }
				}
			) {
				nodes {
					id
					identifier
					title
					description
					url
				}
			}
		}`
	} else {
		query = `
		query {
			issues(
				filter: {
					team: { id: { eq: "` + p.Team + `" } },
					state: { type: { nin: ["completed", "canceled"] } }
				}
			) {
				nodes {
					id
					identifier
					title
					description
					url
				}
			}
		}`
	}

	payload := map[string]interface{}{
		"query": query,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("linear api error: %d %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			Issues struct {
				Nodes []struct {
					ID          string `json:"id"`
					Identifier  string `json:"identifier"`
					Title       string `json:"title"`
					Description string `json:"description"`
					URL         string `json:"url"`
				} `json:"nodes"`
			} `json:"issues"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("linear graphql error: %s", result.Errors[0].Message)
	}

	var items []WorkItem
	for _, node := range result.Data.Issues.Nodes {
		repoURL := extractRepoURL(node.Description, RepoRegex)

		item := WorkItem{
			ID:          node.Identifier,
			Summary:     node.Title,
			Description: node.Description,
			RepoURL:     repoURL, // Will be empty if not found, requires caller to handle or agent to have it configured
			EnvVars: map[string]string{
				"LINEAR_ISSUE_ID": node.ID,
				"LINEAR_ISSUE_KEY": node.Identifier,
				"LINEAR_URL": node.URL,
			},
		}
		items = append(items, item)
	}

	return items, nil
}

// UpdateStatus adds a comment to the Linear issue.
func (p *LinearPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	// Linear uses internal UUIDs for mutations, not the issue identifier (e.g. ENG-123)
	// We need the internal ID. We stored it in EnvVars during Poll.
	issueID, ok := item.EnvVars["LINEAR_ISSUE_ID"]
	if !ok {
		// If we don't have it, we'd need to fetch it first, but for simplicity we rely on the env var
		return fmt.Errorf("linear issue internal ID not found in WorkItem EnvVars")
	}

	if comment != "" {
		if err := p.postComment(ctx, issueID, comment); err != nil {
			return err
		}
	}

	if strings.EqualFold(status, "Done") || strings.EqualFold(status, "Closed") {
		// Moving to completed requires knowing the state ID, which is complex via GraphQL without extra queries.
		// For MVP, we just post a comment that it's done.
		if comment == "" {
			if err := p.postComment(ctx, issueID, "Status changed to: " + status); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *LinearPoller) postComment(ctx context.Context, issueID string, body string) error {
	// Escape the body for JSON
	bodyBytes, _ := json.Marshal(body)
	bodyStr := string(bodyBytes) // already includes quotes

	query := `
	mutation {
		commentCreate(
			input: {
				issueId: "` + issueID + `"
				body: ` + bodyStr + `
			}
		) {
			success
		}
	}`

	payload := map[string]interface{}{
		"query": query,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal mutation: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post linear comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to post linear comment: %d %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Data struct {
			CommentCreate struct {
				Success bool `json:"success"`
			} `json:"commentCreate"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode comment response: %w", err)
	}

	if len(result.Errors) > 0 {
		return fmt.Errorf("linear comment error: %s", result.Errors[0].Message)
	}

	if !result.Data.CommentCreate.Success {
		return fmt.Errorf("linear comment creation failed")
	}

	return nil
}

// Ping verifies the token and team ID are valid.
func (p *LinearPoller) Ping(ctx context.Context) error {
	query := `
	query {
		team(id: "` + p.Team + `") {
			id
		}
	}`

	payload := map[string]interface{}{
		"query": query,
	}
	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	p.setHeaders(req)

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach linear: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("linear ping failed: %d", resp.StatusCode)
	}

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Data struct {
			Team struct {
				ID string `json:"id"`
			} `json:"team"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode ping response: %w", err)
	}

	if len(result.Errors) > 0 {
		return fmt.Errorf("linear ping error: %s", result.Errors[0].Message)
	}

	if result.Data.Team.ID == "" {
		return fmt.Errorf("linear team '%s' not found", p.Team)
	}

	return nil
}

func (p *LinearPoller) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", p.Token)
	req.Header.Set("Content-Type", "application/json")
}
