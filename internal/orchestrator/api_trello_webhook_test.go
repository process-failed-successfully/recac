package orchestrator

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTrelloWebhook(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	baseCtx := context.Background()

	// Ensure Viper config is clear initially
	viper.Reset()
	viper.Set("orchestrator.trello_webhook_secret", "my-trello-secret")

	// Payload that resembles a Trello webhook event for "createCard"
	validPayload := []byte(`{
		"action": {
			"type": "createCard",
			"data": {
				"card": {
					"id": "tr-card-123",
					"name": "Implement feature X",
					"desc": "Please implement X. Repo: https://github.com/test/repo"
				}
			}
		}
	}`)

	ignoredPayload := []byte(`{
		"action": {
			"type": "updateList",
			"data": {}
		}
	}`)

	t.Run("Valid Webhook createCard", func(t *testing.T) {
		mockPoller := new(MockPoller)
		mockSpawner := new(MockSpawner)

		// Expected to spawn a job
		mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
			return item.ID == "tr-card-123" && item.RepoURL == "https://github.com/test/repo"
		})).Return(nil)

		orch := New(mockPoller, mockSpawner, 1*time.Minute)

		mux := http.NewServeMux()
		RegisterAPI(mux, orch, logger, baseCtx)

		req := httptest.NewRequest("POST", "/webhook/trello", bytes.NewBuffer(validPayload))
		// The callback URL in httptest is going to be http://example.com/webhook/trello
		callbackURL := "http://example.com/webhook/trello"

		mac := hmac.New(sha1.New, []byte("my-trello-secret"))
		mac.Write(validPayload)
		mac.Write([]byte(callbackURL))
		expectedMAC := base64.StdEncoding.EncodeToString(mac.Sum(nil))

		req.Header.Set("X-Trello-Webhook", expectedMAC)

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Contains(t, w.Body.String(), "Trello Webhook Job tr-card-123")
		// Need to wait briefly for the goroutine to trigger Spawn
		// Use assert.Eventually to avoid flaky tests
		assert.Eventually(t, func() bool {
			return len(mockSpawner.Calls) > 0
		}, 1*time.Second, 10*time.Millisecond, "mockSpawner.Spawn should be called")
		mockSpawner.AssertExpectations(t)
	})

	t.Run("Invalid Signature", func(t *testing.T) {
		mockPoller := new(MockPoller)
		mockSpawner := new(MockSpawner)
		orch := New(mockPoller, mockSpawner, 1*time.Minute)
		mux := http.NewServeMux()
		RegisterAPI(mux, orch, logger, baseCtx)

		req := httptest.NewRequest("POST", "/webhook/trello", bytes.NewBuffer(validPayload))
		req.Header.Set("X-Trello-Webhook", "invalid-sig")

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid X-Trello-Webhook signature")
	})

	t.Run("Missing Signature", func(t *testing.T) {
		mockPoller := new(MockPoller)
		mockSpawner := new(MockSpawner)
		orch := New(mockPoller, mockSpawner, 1*time.Minute)
		mux := http.NewServeMux()
		RegisterAPI(mux, orch, logger, baseCtx)

		req := httptest.NewRequest("POST", "/webhook/trello", bytes.NewBuffer(validPayload))

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Missing X-Trello-Webhook header")
	})

	t.Run("Ignored Action Type", func(t *testing.T) {
		mockPoller := new(MockPoller)
		mockSpawner := new(MockSpawner)
		orch := New(mockPoller, mockSpawner, 1*time.Minute)
		mux := http.NewServeMux()
		RegisterAPI(mux, orch, logger, baseCtx)

		req := httptest.NewRequest("POST", "/webhook/trello", bytes.NewBuffer(ignoredPayload))
		callbackURL := "http://example.com/webhook/trello"

		mac := hmac.New(sha1.New, []byte("my-trello-secret"))
		mac.Write(ignoredPayload)
		mac.Write([]byte(callbackURL))
		expectedMAC := base64.StdEncoding.EncodeToString(mac.Sum(nil))

		req.Header.Set("X-Trello-Webhook", expectedMAC)

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		// Ignored actions return 200 OK to Trello
		assert.Equal(t, http.StatusOK, w.Code)
		mockSpawner.AssertNotCalled(t, "Spawn")
	})

	t.Run("HEAD Request", func(t *testing.T) {
		mockPoller := new(MockPoller)
		mockSpawner := new(MockSpawner)
		orch := New(mockPoller, mockSpawner, 1*time.Minute)
		mux := http.NewServeMux()
		RegisterAPI(mux, orch, logger, baseCtx)

		req := httptest.NewRequest("HEAD", "/webhook/trello", nil)

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
