package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookNotifier_Send(t *testing.T) {
	secret := "test-secret"
	expectedEvent := "on_success"
	expectedMessage := "Job completed successfully"

	// Create a mock HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected method POST, got %s", r.Method)
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		var payload WebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("Failed to unmarshal payload: %v", err)
		}

		if payload.Event != expectedEvent {
			t.Errorf("Expected event %s, got %s", expectedEvent, payload.Event)
		}
		if payload.Message != expectedMessage {
			t.Errorf("Expected message %s, got %s", expectedMessage, payload.Message)
		}

		// Verify HMAC signature
		signatureHeader := r.Header.Get("X-Webhook-Signature")
		if signatureHeader == "" {
			t.Errorf("Expected X-Webhook-Signature header, got none")
		} else {
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
			if signatureHeader != expectedSignature {
				t.Errorf("Expected signature %s, got %s", expectedSignature, signatureHeader)
			}
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	notifier := NewWebhookNotifier(ts.URL, secret)
	err := notifier.Send(context.Background(), expectedEvent, expectedMessage)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestWebhookNotifier_Send_NoSecret(t *testing.T) {
	// Create a mock HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signatureHeader := r.Header.Get("X-Webhook-Signature")
		if signatureHeader != "" {
			t.Errorf("Expected no X-Webhook-Signature header, got %s", signatureHeader)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	notifier := NewWebhookNotifier(ts.URL, "")
	err := notifier.Send(context.Background(), "on_start", "Starting job")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestWebhookNotifier_Send_Error(t *testing.T) {
	// Create a mock HTTP server returning an error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	notifier := NewWebhookNotifier(ts.URL, "")
	err := notifier.Send(context.Background(), "on_failure", "Failed job")
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
}
