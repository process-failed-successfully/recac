package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type WebhookListener struct {
	orch   *Orchestrator
	server *http.Server
	logger *slog.Logger
}

func NewWebhookListener(orch *Orchestrator, addr string, logger *slog.Logger) *WebhookListener {
	mux := http.NewServeMux()
	wl := &WebhookListener{
		orch:   orch,
		logger: logger,
	}
	mux.HandleFunc("/webhook", wl.handleWebhook)

	wl.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return wl
}

func (wl *WebhookListener) Start(ctx context.Context) error {
	wl.logger.Info("Starting Webhook Listener", "addr", wl.server.Addr)

	// Start server in goroutine
	go func() {
		if err := wl.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			wl.logger.Error("Webhook listener failed", "error", err)
		}
	}()

	// Wait for context cancellation to shutdown
	<-ctx.Done()
	wl.logger.Info("Shutting down Webhook Listener...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return wl.server.Shutdown(shutdownCtx)
}

func (wl *WebhookListener) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Basic check for GitHub Event header
	event := r.Header.Get("X-GitHub-Event")
	// If the header is missing, we might assume it's a generic webhook or try to parse anyway.
	// But enforcing it filters noise. However, Jira webhooks don't send X-GitHub-Event.
	// For this MVP, we focus on GitHub, so we check if it is set.
	if event != "" && event != "issues" {
		wl.logger.Info("Ignored webhook event", "event", event)
		w.WriteHeader(http.StatusOK)
		return
	}

	var payload struct {
		Action string `json:"action"`
		Issue  struct {
			Number  int    `json:"number"`
			Title   string `json:"title"`
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
		} `json:"issue"`
		Repository struct {
			HTMLURL string `json:"html_url"`
		} `json:"repository"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		wl.logger.Error("Failed to decode webhook payload", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Check if this looks like a GitHub Issue event
	if payload.Issue.HTMLURL == "" {
		// Maybe it's not a GitHub payload.
		// For now, log and ignore.
		wl.logger.Warn("Received webhook without issue details")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// We filter for "opened" or "reopened"
	if payload.Action != "opened" && payload.Action != "reopened" {
		wl.logger.Info("Ignored issue action", "action", payload.Action)
		w.WriteHeader(http.StatusOK)
		return
	}

	item := WorkItem{
		ID:          fmt.Sprintf("gh-%d", payload.Issue.Number),
		Summary:     payload.Issue.Title,
		Description: payload.Issue.Body,
		RepoURL:     payload.Repository.HTMLURL,
		EnvVars: map[string]string{
			"GITHUB_ISSUE": strconv.Itoa(payload.Issue.Number),
			"SOURCE":       "webhook",
		},
	}

	wl.logger.Info("Received webhook work item", "id", item.ID)

	// Add to orchestrator asynchronously to prevent blocking the webhook
	go func() {
		wl.orch.AddWork(item)
	}()

	w.WriteHeader(http.StatusAccepted)
}
