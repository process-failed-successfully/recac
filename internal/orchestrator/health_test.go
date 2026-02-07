package orchestrator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckAIProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		apiKey   string
		wantErr  bool
	}{
		{
			name:     "Valid Key",
			provider: "openai",
			apiKey:   "sk-test-1234567890",
			wantErr:  false,
		},
		{
			name:     "Missing Key",
			provider: "anthropic",
			apiKey:   "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckAIProvider(tt.provider, tt.apiKey); (err != nil) != tt.wantErr {
				t.Errorf("CheckAIProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckGitHub(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Headers
		if r.Header.Get("Authorization") != "Bearer valid-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Verify Path
		if r.URL.Path == "/repos/owner/repo" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Override base URL and client for testing
	origBaseURL := githubBaseURL
	origClient := httpClient
	githubBaseURL = server.URL
	httpClient = server.Client()
	defer func() {
		githubBaseURL = origBaseURL
		httpClient = origClient
	}()

	ctx := context.Background()

	tests := []struct {
		name    string
		token   string
		owner   string
		repo    string
		wantErr bool
	}{
		{
			name:    "Valid Credentials",
			token:   "valid-token",
			owner:   "owner",
			repo:    "repo",
			wantErr: false,
		},
		{
			name:    "Invalid Token",
			token:   "invalid-token",
			owner:   "owner",
			repo:    "repo",
			wantErr: true,
		},
		{
			name:    "Repo Not Found",
			token:   "valid-token",
			owner:   "owner",
			repo:   "missing-repo",
			wantErr: true,
		},
		{
			name:    "Missing Token",
			token:   "",
			owner:   "owner",
			repo:    "repo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckGitHub(ctx, tt.token, tt.owner, tt.repo); (err != nil) != tt.wantErr {
				t.Errorf("CheckGitHub() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
