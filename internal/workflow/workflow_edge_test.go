package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recac/internal/jira"
	"recac/internal/telemetry"

	"github.com/stretchr/testify/assert"
)

// TestProcessJiraTicket_InvalidFormat checks when ticket JSON is missing fields
func TestProcessJiraTicket_InvalidFormat(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/INVALID-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "INVALID-1",
			// fields is missing
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{
		Logger: telemetry.NewLogger(true, "", false),
	}

	err := ProcessJiraTicket(context.Background(), "INVALID-1", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ticket format")
}

// TestRunWorkflow_SessionManagerCreationError checks when creating SessionManager fails
func TestRunWorkflow_SessionManagerCreationError(t *testing.T) {
	original := NewSessionManagerFunc
	defer func() { NewSessionManagerFunc = original }()

	NewSessionManagerFunc = func() (ISessionManager, error) {
		return nil, assert.AnError
	}

	cfg := SessionConfig{
		Detached:    true,
		SessionName: "test-detached",
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	// We expect either session manager creation error or executable error.
}
