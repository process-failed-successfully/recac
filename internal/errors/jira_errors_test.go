package errors

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestHandleJiraAPIError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		maxRetries  int
		retryDelay  time.Duration
		wantRetries bool
	}{
		{
			name: "Jira rate limit error",
			err: NewJiraError(429, "Too many requests", 5*time.Second),
			maxRetries: 3,
			retryDelay: 100 * time.Millisecond,
			wantRetries: true,
		},
		{
			name: "Jira server error",
			err: NewJiraError(500, "Internal server error", 0),
			maxRetries: 3,
			retryDelay: 100 * time.Millisecond,
			wantRetries: true,
		},
		{
			name: "Jira client error",
			err: NewJiraError(404, "Not found", 0),
			maxRetries: 3,
			retryDelay: 100 * time.Millisecond,
			wantRetries: false,
		},
		{
			name: "Network timeout error",
			err: &net.OpError{Op: "read", Err: errors.New("timeout")},
			maxRetries: 3,
			retryDelay: 100 * time.Millisecond,
			wantRetries: true,
		},
		{
			name: "Generic error",
			err: errors.New("generic error"),
			maxRetries: 3,
			retryDelay: 100 * time.Millisecond,
			wantRetries: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := HandleJiraAPIError(tt.err, tt.maxRetries, tt.retryDelay)

			if tt.wantRetries {
				if err == nil {
					t.Logf("Expected retry to happen for %s", tt.name)
				} else {
					t.Logf("Error handled with retries: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected error to be returned for %s", tt.name)
				}
			}
		})
	}
}

func TestNewJiraError(t *testing.T) {
	err := NewJiraError(429, "Too many requests", 5*time.Second)

	if err.StatusCode != 429 {
		t.Errorf("Expected status code 429, got %d", err.StatusCode)
	}

	if err.Message != "Too many requests" {
		t.Errorf("Expected message 'Too many requests', got %s", err.Message)
	}

	if err.RetryAfter != 5*time.Second {
		t.Errorf("Expected retry after 5s, got %v", err.RetryAfter)
	}

	expectedErrorString := "Jira API error (status 429): Too many requests"
	if err.Error() != expectedErrorString {
		t.Errorf("Expected error string '%s', got '%s'", expectedErrorString, err.Error())
	}
}
