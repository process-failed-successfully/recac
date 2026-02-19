package workflow

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"

	"github.com/stretchr/testify/assert"
)

func TestProcessJiraTicket_Error_GetTicket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{}

	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)
	assert.Error(t, err)
}

func TestProcessJiraTicket_Error_SetupWorkspace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"key":"TEST-1",
			"fields":{
				"summary":"S",
				"description": "D"
			}
		}`))
	}))
	defer server.Close()

	jClient := jira.NewClient(server.URL, "user", "token")

	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("setup failed")
	}

	cfg := SessionConfig{
		RepoURL:     "https://github.com/test/repo",
		ProjectPath: "/tmp/test",
	}

	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)
	assert.Error(t, err)
	assert.Equal(t, "setup failed", err.Error())
}

func TestProcessDirectTask_Error_SetupWorkspace(t *testing.T) {
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		return "", errors.New("setup failed")
	}

	cfg := SessionConfig{
		RepoURL: "https://github.com/test/repo",
	}

	err := ProcessDirectTask(context.Background(), cfg)
	assert.Error(t, err)
	assert.Equal(t, "setup failed", err.Error())
}

func TestProcessJiraTicket_Blocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return ticket with blockers
		w.Write([]byte(`{
			"key":"TEST-1",
			"fields":{
				"summary":"S",
				"description": "D",
				"issuelinks": [
					{
						"type": { "inward": "is blocked by" },
						"inwardIssue": {
							"key": "BLOCK-1",
							"fields": {
								"status": { "name": "Open" }
							}
						}
					}
				]
			}
		}`))
	}))
	defer server.Close()

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{
		Logger: nil,
	}

	// Should return nil (skipped)
	err := ProcessJiraTicket(context.Background(), "TEST-1", jClient, cfg, nil)
	assert.NoError(t, err)
}
