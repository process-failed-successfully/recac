package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

// WebhookPoller implements the Poller interface for receiving work via webhooks.
type WebhookPoller struct {
	server *http.Server
	queue  []WorkItem
	mu     sync.Mutex
	logger *slog.Logger
}

// NewWebhookPoller creates a new WebhookPoller listening on the specified address.
func NewWebhookPoller(addr string, logger *slog.Logger) *WebhookPoller {
	wp := &WebhookPoller{
		queue:  make([]WorkItem, 0),
		logger: logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", wp.handleWebhook)

	wp.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		logger.Info("Starting webhook server", "addr", addr)
		if err := wp.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Webhook server failed", "error", err)
		}
	}()

	return wp
}

func (p *WebhookPoller) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var item WorkItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate minimal fields
	if item.ID == "" || item.RepoURL == "" {
		http.Error(w, "Missing required fields: id, repo_url", http.StatusBadRequest)
		return
	}

	p.mu.Lock()
	p.queue = append(p.queue, item)
	p.mu.Unlock()

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, "Accepted item %s", item.ID)
}

// Poll returns all items currently in the queue.
func (p *WebhookPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.queue) == 0 {
		return nil, nil
	}

	items := make([]WorkItem, len(p.queue))
	copy(items, p.queue)
	p.queue = make([]WorkItem, 0) // Clear queue

	return items, nil
}

// UpdateStatus logs the status update.
func (p *WebhookPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	// Webhooks are one-way, so we just log the update.
	// In a real implementation, we might want to send a callback if the item has a CallbackURL.
	if p.logger != nil {
		p.logger.Info("Webhook Item Status Update", "id", item.ID, "status", status, "comment", comment)
	}
	return nil
}
