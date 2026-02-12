package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWebhookPoller(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	// Use port 0 to get random free port
	poller, err := NewWebhookPoller(0, logger)
	assert.NoError(t, err)
	defer poller.server.Shutdown(context.Background())

	// Allow server to start
	time.Sleep(100 * time.Millisecond)

	// Send a webhook request
	item := WorkItem{
		ID:          "WH-1",
		Summary:     "Webhook Task",
		Description: "Do something",
		RepoURL:     "https://github.com/test/repo",
	}
	body, _ := json.Marshal(item)
	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/webhook", poller.Port), "application/json", bytes.NewBuffer(body))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	// Poll
	ctx := context.Background()
	items, err := poller.Poll(ctx, logger)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "WH-1", items[0].ID)
	assert.Equal(t, "webhook", items[0].Source)

	// Poll again (should be empty)
	items, err = poller.Poll(ctx, logger)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestWebhookPoller_InvalidMethod(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	poller, err := NewWebhookPoller(0, logger)
	assert.NoError(t, err)
	defer poller.server.Shutdown(context.Background())
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/webhook", poller.Port))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestWebhookPoller_MissingID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	poller, err := NewWebhookPoller(0, logger)
	assert.NoError(t, err)
	defer poller.server.Shutdown(context.Background())
	time.Sleep(100 * time.Millisecond)

	item := WorkItem{
		Summary: "No ID",
	}
	body, _ := json.Marshal(item)
	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/webhook", poller.Port), "application/json", bytes.NewBuffer(body))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
