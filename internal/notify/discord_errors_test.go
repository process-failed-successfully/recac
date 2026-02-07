package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscordNotifier_Send_NoConfig(t *testing.T) {
	n := &DiscordNotifier{}
	if _, err := n.Send(context.Background(), "msg", ""); err == nil {
		t.Error("expected error for no config, got nil")
	}
}

func TestDiscordNotifier_Send_Webhook_Errors(t *testing.T) {
	// Case 1: Status Error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	n := NewDiscordNotifier(server.URL)
	if _, err := n.Send(context.Background(), "msg", ""); err == nil {
		t.Error("expected error for status 500, got nil")
	}
}

func TestDiscordNotifier_Send_Webhook_NetworkError(t *testing.T) {
	n := NewDiscordNotifier("http://invalid-url")
	n.Client.Transport = &errorTransport{} // Reusing errorTransport from slack_test.go

	if _, err := n.Send(context.Background(), "msg", ""); err == nil {
		t.Error("expected error for network failure, got nil")
	}
}

func TestDiscordNotifier_Send_Bot_Errors(t *testing.T) {
	// Case 1: Status Error with Body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message": "Bad Request"}`))
	}))
	defer server.Close()

	n := NewDiscordBotNotifier("token", "chan")
	n.Client.Transport = &testTransport{TargetURL: server.URL} // testTransport is in discord_test.go

	_, err := n.Send(context.Background(), "msg", "")
	if err == nil {
		t.Error("expected error, got nil")
	} else if !strings.Contains(err.Error(), "Bad Request") {
		t.Errorf("expected error details, got %v", err)
	}

	// Case 2: Invalid JSON Response
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid`))
	}))
	defer server2.Close()

	n2 := NewDiscordBotNotifier("token", "chan")
	n2.Client.Transport = &testTransport{TargetURL: server2.URL}

	if _, err := n2.Send(context.Background(), "msg", ""); err == nil {
		t.Error("expected error for invalid json, got nil")
	}
}

func TestDiscordNotifier_AddReaction_Errors(t *testing.T) {
	// Case 1: No Config
	n := &DiscordNotifier{}
	if err := n.AddReaction(context.Background(), "msg", "emoji"); err == nil {
		t.Error("expected error for no config, got nil")
	}

	// Case 2: Network Error
	n = NewDiscordBotNotifier("token", "chan")
	n.Client.Transport = &errorTransport{}

	if err := n.AddReaction(context.Background(), "msg", "emoji"); err == nil {
		t.Error("expected error for network failure, got nil")
	}
}

func TestMapEmoji(t *testing.T) {
	tests := []struct{
		input string
		want string
	}{
		{"white_check_mark", "%E2%9C%85"},
		{":x:", "%E2%9D%8C"},
		{"warning", "%E2%9A%A0%EF%B8%8F"},
		{"other", "other"},
	}

	for _, tt := range tests {
		if got := mapEmoji(tt.input); got != tt.want {
			t.Errorf("mapEmoji(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
