package orchestrator

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWebhookListener_HandleWebhook_GitHub(t *testing.T) {
	// Setup
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 1*time.Minute)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wl := NewWebhookListener(orch, ":0", logger)

	// Create Request
	payload := `{
		"action": "opened",
		"issue": {
			"number": 123,
			"title": "Bug Report",
			"body": "Fix this bug",
			"html_url": "https://github.com/owner/repo/issues/123"
		},
		"repository": {
			"html_url": "https://github.com/owner/repo"
		}
	}`
	req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issues")

	w := httptest.NewRecorder()

	// Handle
	wl.handleWebhook(w, req)

	// Assert Response
	assert.Equal(t, http.StatusAccepted, w.Code)

	// Assert Orchestrator received item
	select {
	case item := <-orch.workCh:
		assert.Equal(t, "gh-123", item.ID)
		assert.Equal(t, "Bug Report", item.Summary)
		assert.Equal(t, "Fix this bug", item.Description)
		assert.Equal(t, "https://github.com/owner/repo", item.RepoURL)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Orchestrator did not receive work item")
	}
}

func TestWebhookListener_HandleWebhook_IgnoreAction(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 1*time.Minute)
	wl := NewWebhookListener(orch, ":0", silentLogger)

	payload := `{"action": "closed", "issue": {"html_url": "foo"}}`
	req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString(payload))
	req.Header.Set("X-GitHub-Event", "issues")
	w := httptest.NewRecorder()

	wl.handleWebhook(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	select {
	case <-orch.workCh:
		t.Fatal("Should not receive work item for closed action")
	default:
		// Pass
	}
}

func TestWebhookListener_HandleWebhook_InvalidPayload(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 1*time.Minute)
	wl := NewWebhookListener(orch, ":0", silentLogger)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	wl.handleWebhook(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebhookListener_Start_Shutdown(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 1*time.Minute)
	wl := NewWebhookListener(orch, ":0", silentLogger)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error)
	go func() {
		// Use a high port or let OS assign
		// Since Start uses wl.server.Addr, and New sets it to ":0",
		// ListenAndServe usually works but getting the actual port requires checking the listener.
		// Start() starts the server.
		errCh <- wl.Start(ctx)
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	cancel()

	err := <-errCh
	// Http server shutdown returns nil usually
	assert.NoError(t, err)
}
