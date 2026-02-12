package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebhookPoller_HandleWebhook(t *testing.T) {
	poller := &WebhookPoller{
		queue:  make([]WorkItem, 0),
		logger: slog.Default(),
	}

	t.Run("Valid Item", func(t *testing.T) {
		item := WorkItem{
			ID:      "TEST-1",
			Summary: "Test Item",
			RepoURL: "https://github.com/test/repo",
		}
		body, _ := json.Marshal(item)
		req := httptest.NewRequest("POST", "/webhook", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		poller.handleWebhook(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		poller.mu.Lock()
		defer poller.mu.Unlock()
		assert.Len(t, poller.queue, 1)
		assert.Equal(t, "TEST-1", poller.queue[0].ID)

		// Reset queue for next test
		poller.queue = make([]WorkItem, 0)
	})

	t.Run("Missing ID", func(t *testing.T) {
		item := WorkItem{
			Summary: "Test Item",
			RepoURL: "https://github.com/test/repo",
		}
		body, _ := json.Marshal(item)
		req := httptest.NewRequest("POST", "/webhook", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		poller.handleWebhook(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString("{invalid-json"))
		w := httptest.NewRecorder()

		poller.handleWebhook(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Wrong Method", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/webhook", nil)
		w := httptest.NewRecorder()

		poller.handleWebhook(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})
}

func TestWebhookPoller_Poll(t *testing.T) {
	poller := &WebhookPoller{
		queue: []WorkItem{
			{ID: "1"},
			{ID: "2"},
		},
		logger: slog.Default(),
	}

	items, err := poller.Poll(context.Background(), slog.Default())
	assert.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "1", items[0].ID)
	assert.Equal(t, "2", items[1].ID)

	// Verify queue is empty
	poller.mu.Lock()
	assert.Len(t, poller.queue, 0)
	poller.mu.Unlock()

	// Poll again should be empty
	items, err = poller.Poll(context.Background(), slog.Default())
	assert.NoError(t, err)
	assert.Len(t, items, 0)
}

func TestNewWebhookPoller(t *testing.T) {
	logger := slog.Default()
	poller := NewWebhookPoller(":0", logger)
	assert.NotNil(t, poller)
	assert.NotNil(t, poller.server)
	// Clean up to prevent leaks
	poller.server.Close()
}
