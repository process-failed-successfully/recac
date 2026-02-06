package github

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

// Client implements IIssueTracker for GitHub.
type Client struct {
	BaseURL    string
	Token      string
	Owner      string
	Repo       string
	HTTPClient *http.Client
}

// NewClient creates a new GitHub client.
func NewClient(token, owner, repo string) *Client {
	return &Client{
		BaseURL: "https://api.github.com",
		Token:   token,
		Owner:   owner,
		Repo:    repo,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Authenticate verifies the token works by getting the user.
func (c *Client) Authenticate(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/user", nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed: %d %s", resp.StatusCode, string(body))
	}
	return nil
}

// CreateTicket creates a new GitHub issue.
// projectKey is ignored (uses c.Owner/c.Repo).
func (c *Client) CreateTicket(ctx context.Context, projectKey, summary, description, issueType string, labels []string) (string, error) {
	// Map issueType to label
	typeLabel := fmt.Sprintf("kind/%s", strings.ToLower(issueType))
	allLabels := append(labels, typeLabel)

	payload := map[string]interface{}{
		"title":  summary,
		"body":   description,
		"labels": allLabels,
	}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/issues", c.BaseURL, c.Owner, c.Repo)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create issue: %d %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	numberVal, ok := result["number"].(float64)
	if !ok {
		return "", fmt.Errorf("response missing issue number")
	}

	return fmt.Sprintf("GH-%d", int(numberVal)), nil
}

// CreateChildTicket creates a new GitHub issue with a reference to the parent.
func (c *Client) CreateChildTicket(ctx context.Context, projectKey, summary, description, issueType, parentKey string, labels []string) (string, error) {
	// Append parent reference to description
	parentNum := strings.TrimPrefix(parentKey, "GH-")
	description += fmt.Sprintf("\n\nParent: #%s", parentNum)

	return c.CreateTicket(ctx, projectKey, summary, description, issueType, labels)
}

// AddIssueLink adds a comment to link issues.
func (c *Client) AddIssueLink(ctx context.Context, inwardKey, outwardKey, linkType string) error {
	// outwardKey is the one being linked FROM (the "blocked" one)
	// inwardKey is the one being linked TO (the "blocker")
	// linkType is "Blocks" usually.

	// We want to add a comment to outwardKey saying "Blocked by #inwardKey"

	outwardNum := strings.TrimPrefix(outwardKey, "GH-")
	inwardNum := strings.TrimPrefix(inwardKey, "GH-")

	var comment string
	if linkType == "Blocks" {
		comment = fmt.Sprintf("Blocked by #%s", inwardNum)
	} else {
		comment = fmt.Sprintf("Relates to #%s", inwardNum)
	}

	return c.postComment(ctx, outwardNum, comment)
}

func (c *Client) postComment(ctx context.Context, issueNum, body string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%s/comments", c.BaseURL, c.Owner, c.Repo, issueNum)

	payload := map[string]string{"body": body}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to post comment: %d %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "token "+c.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "recac-github-client")
}
