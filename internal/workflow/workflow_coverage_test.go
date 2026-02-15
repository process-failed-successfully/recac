package workflow

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"recac/internal/jira"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// CoverageMockSessionManager
type CoverageMockSessionManager struct {
	mock.Mock
}

func (m *CoverageMockSessionManager) StartSession(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
	args := m.Called(name, goal, command, cwd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*runner.SessionState), args.Error(1)
}

func TestRunWorkflow_Detached_Success_Coverage(t *testing.T) {
	mockSM := new(CoverageMockSessionManager)

	mockSM.On("StartSession", "test-session", "test-goal", mock.Anything, mock.Anything).
		Return(&runner.SessionState{PID: 12345, LogFile: "log.txt"}, nil)

	cfg := SessionConfig{
		Detached:       true,
		SessionName:    "test-session",
		Goal:           "test-goal",
		SessionManager: mockSM,
		CommandPrefix:  []string{"start"},
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.NoError(t, err)

	mockSM.AssertExpectations(t)
}

func TestRunWorkflow_Detached_Error_Coverage(t *testing.T) {
	mockSM := new(CoverageMockSessionManager)

	mockSM.On("StartSession", "test-session", "test-goal", mock.Anything, mock.Anything).
		Return(nil, errors.New("start failed"))

	cfg := SessionConfig{
		Detached:       true,
		SessionName:    "test-session",
		Goal:           "test-goal",
		SessionManager: mockSM,
		CommandPrefix:  []string{"start"},
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start failed")
}

func TestRunWorkflow_Detached_MissingName_Coverage(t *testing.T) {
	cfg := SessionConfig{
		Detached: true,
		// Missing SessionName
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--name is required")
}

func TestProcessJiraTicket_FetchError_Coverage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{
		Logger: nil, // will use default
	}

	err := ProcessJiraTicket(context.Background(), "TICKET-1", client, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch ticket")
}

func TestProcessJiraTicket_Blocked_Coverage(t *testing.T) {
	// Mock Blocked Ticket
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/TICKET-1" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"key": "TICKET-1",
				"fields": {
					"issuelinks": [
						{
							"type": { "inward": "is blocked by" },
							"inwardIssue": {
								"key": "BLOCK-1",
								"fields": { "status": { "name": "Open" } }
							}
						}
					]
				}
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{}

	// Should return nil (skipped)
	err := ProcessJiraTicket(context.Background(), "TICKET-1", client, cfg, map[string]bool{})
	assert.NoError(t, err)
}

func TestProcessJiraTicket_InvalidFormat_Coverage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Return JSON without fields
		w.Write([]byte(`{"key": "TICKET-1"}`))
	}))
	defer server.Close()

	client := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{}

	err := ProcessJiraTicket(context.Background(), "TICKET-1", client, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ticket format")
}

func TestProcessJiraTicket_NoRepoURL_Coverage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"key": "TICKET-1",
			"fields": {
				"summary": "Summary",
				"description": {
					"type": "doc",
					"content": [{"type": "paragraph", "content": [{"type": "text", "text": "No repo here"}]}]
				},
				"issuelinks": []
			}
		}`))
	}))
	defer server.Close()

	client := jira.NewClient(server.URL, "user", "token")

	tmpDir, _ := os.MkdirTemp("", "workflow-test")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
	}

	err := ProcessJiraTicket(context.Background(), "TICKET-1", client, cfg, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no repo url found")
}
