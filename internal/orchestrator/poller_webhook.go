package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
)

// WebhookPoller listens for work items via HTTP POST.
type WebhookPoller struct {
	server *http.Server
	items  []WorkItem
	mu     sync.Mutex
	logger *slog.Logger
	Port   int
}

// NewWebhookPoller creates a new WebhookPoller listening on the specified port.
func NewWebhookPoller(port int, logger *slog.Logger) (*WebhookPoller, error) {
	wp := &WebhookPoller{
		items:  make([]WorkItem, 0),
		logger: logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", wp.handleWebhook)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	wp.Port = listener.Addr().(*net.TCPAddr).Port
	wp.server = &http.Server{
		Handler: mux,
	}

	// Start server in background
	go func() {
		logger.Info("Starting Webhook Poller server", "port", wp.Port)
		if err := wp.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Error("Webhook server failed", "error", err)
		}
	}()

	return wp, nil
}

func (wp *WebhookPoller) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var item WorkItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if item.ID == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	item.Source = "webhook"

	wp.mu.Lock()
	wp.items = append(wp.items, item)
	wp.mu.Unlock()

	wp.logger.Info("Received work item via webhook", "id", item.ID)
	w.WriteHeader(http.StatusAccepted)
}

func (wp *WebhookPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if len(wp.items) == 0 {
		return nil, nil
	}

	// Return all items and clear buffer
	items := wp.items
	wp.items = make([]WorkItem, 0)
	return items, nil
}

func (wp *WebhookPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	wp.logger.Info("[WebhookPoller] Status update", "id", item.ID, "status", status, "comment", comment)
	return nil
}
