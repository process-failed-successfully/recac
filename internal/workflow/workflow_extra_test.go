package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"recac/internal/jira"
	"github.com/stretchr/testify/assert"
)

func TestProcessJiraTicket_Blocked(t *testing.T) {
	// Mock Jira Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket with Blocker
	mux.HandleFunc("/rest/api/3/issue/TEST-BLOCKED", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-BLOCKED",
			"fields": map[string]interface{}{
				"summary": "Blocked Ticket",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{},
				},
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{"name": "Blocks", "inward": "is blocked by"},
						"inwardIssue": map[string]interface{}{
							"key": "BLOCKER-1",
							"fields": map[string]interface{}{
								"status": map[string]interface{}{"name": "Open"},
							},
						},
					},
				},
			},
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")

	cfg := SessionConfig{
		IsMock: true,
	}

	// Should return nil (skipped)
	err := ProcessJiraTicket(context.Background(), "TEST-BLOCKED", jClient, cfg, nil)
	assert.NoError(t, err)
}

func TestProcessJiraTicket_NoRepoURL(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/TEST-NOREPO", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-NOREPO",
			"fields": map[string]interface{}{
				"summary": "No Repo Ticket",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{},
				},
				"issuelinks": []interface{}{},
			},
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{IsMock: true} // RepoURL empty

	err := ProcessJiraTicket(context.Background(), "TEST-NOREPO", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no repo url found")
}

func TestProcessJiraTicket_WorkspaceFail(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/TEST-WS", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "TEST-WS",
			"fields": map[string]interface{}{
				"summary": "Ticket",
				"description": map[string]interface{}{},
				"issuelinks": []interface{}{},
			},
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")

	// Use invalid path to trigger MkdirAll error
	// On Linux/Unix, /proc is usually read-only or we can use a file as dir
	tmpFile, _ := os.CreateTemp("", "file")
	defer os.Remove(tmpFile.Name())

	cfg := SessionConfig{
		ProjectPath: tmpFile.Name(), // File exists, so MkdirAll fails
		IsMock: true,
	}

	err := ProcessJiraTicket(context.Background(), "TEST-WS", jClient, cfg, nil)
	assert.Error(t, err)
	// Error message might vary depending on OS, but it should fail
}
