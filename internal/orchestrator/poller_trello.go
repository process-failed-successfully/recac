package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

// TrelloPoller implements the Poller interface for Trello boards/lists.
type TrelloPoller struct {
	BaseURL string
	Key     string
	Token   string
	BoardID string
	ListID  string
	Client  *http.Client
}

// NewTrelloPoller creates a new TrelloPoller.
func NewTrelloPoller(key, token, boardID, listID string) *TrelloPoller {
	return &TrelloPoller{
		BaseURL: "https://api.trello.com/1",
		Key:     key,
		Token:   token,
		BoardID: boardID,
		ListID:  listID, // Used to filter or fetch from a specific list
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Ensure TrelloPoller implements Poller
var _ Poller = (*TrelloPoller)(nil)

func (p *TrelloPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	// If a ListID is provided, fetch cards from that list.
	// Otherwise, fetch open cards from the board.
	var url string
	if p.ListID != "" {
		url = fmt.Sprintf("%s/lists/%s/cards?key=%s&token=%s", p.BaseURL, p.ListID, p.Key, p.Token)
	} else if p.BoardID != "" {
		url = fmt.Sprintf("%s/boards/%s/cards?key=%s&token=%s", p.BaseURL, p.BoardID, p.Key, p.Token)
	} else {
		return nil, fmt.Errorf("either BoardID or ListID must be provided")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch trello cards: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("trello api error: %d %s", resp.StatusCode, string(body))
	}

	var cards []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&cards); err != nil {
		return nil, fmt.Errorf("failed to decode trello response: %w", err)
	}

	var items []WorkItem
	for _, card := range cards {
		closed, _ := card["closed"].(bool)
		if closed {
			continue // Skip archived cards
		}

		id, _ := card["id"].(string)
		name, _ := card["name"].(string)
		desc, _ := card["desc"].(string)

		// Check labels for filtering if ListID is empty but we want to filter by something (not implemented yet, but keeping scope simple)
		// For now, if ListID is given, it's already filtered. If BoardID is given, we process all open cards.

		repoURL := extractRepoURL(desc, RepoRegex)

		item := WorkItem{
			ID:          id,
			Summary:     name,
			Description: desc,
			RepoURL:     repoURL,
			EnvVars: map[string]string{
				"TRELLO_CARD_ID": id,
			},
		}

		items = append(items, item)
	}

	return items, nil
}

func (p *TrelloPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	// 1. Post Comment
	if comment != "" {
		if err := p.postComment(ctx, item.ID, comment); err != nil {
			return err
		}
	}

	// 2. Archive if Done
	// Trello doesn't have a single "Done" state natively unless we move to a list.
	// For simplicity, we can close (archive) the card.
	if strings.EqualFold(status, "Done") || strings.EqualFold(status, "Closed") {
		return p.closeCard(ctx, item.ID)
	}

	return nil
}

func (p *TrelloPoller) postComment(ctx context.Context, cardID, text string) error {
	url := fmt.Sprintf("%s/cards/%s/actions/comments?key=%s&token=%s&text=%s", p.BaseURL, cardID, p.Key, p.Token, neturl.QueryEscape(text))

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post trello comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to post trello comment: %d %s", resp.StatusCode, string(body))
	}
	return nil
}

func (p *TrelloPoller) closeCard(ctx context.Context, cardID string) error {
	url := fmt.Sprintf("%s/cards/%s?key=%s&token=%s&closed=true", p.BaseURL, cardID, p.Key, p.Token)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, nil)
	if err != nil {
		return err
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to close trello card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to close trello card: %d %s", resp.StatusCode, string(body))
	}
	return nil
}

func (p *TrelloPoller) Ping(ctx context.Context) error {
	// Simple query to verify connectivity and credentials
	// We'll just fetch the member associated with the token
	url := fmt.Sprintf("%s/members/me?key=%s&token=%s", p.BaseURL, p.Key, p.Token)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("trello connectivity check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("trello ping failed with status: %d", resp.StatusCode)
	}
	return nil
}
