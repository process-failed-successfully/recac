package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProcessJiraTicket_ErrorHandling(t *testing.T) {
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		// For these unit tests, we are not testing the full workflow run.
		return nil
	}

	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mocks
	mux.HandleFunc("/rest/api/3/issue/GOOD-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "GOOD-1",
			"fields": map[string]interface{}{
				"summary": "Test Ticket",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{
						{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": "Repo: https://github.com/example/repo"}}},
					},
				},
				"issuelinks": []interface{}{},
			},
		})
	})
	mux.HandleFunc("/rest/api/3/issue/BLOCKED-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "BLOCKED-1",
			"fields": map[string]interface{}{
				"summary": "Blocked Ticket",
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{"inward": "is blocked by"},
						"inwardIssue": map[string]interface{}{
							"key":    "BLOCKER-1",
							"fields": map[string]interface{}{"status": map[string]interface{}{"name": "In Progress"}},
						},
					},
				},
			},
		})
	})
	mux.HandleFunc("/rest/api/3/issue/NO-REPO-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"key": "NO-REPO-1", "fields": map[string]interface{}{"summary": "No Repo"}})
	})
	mux.HandleFunc("/rest/api/3/issue/FAIL-TRANSITION-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "FAIL-TRANSITION-1",
			"fields": map[string]interface{}{
				"summary": "Fail Transition",
				"description": map[string]interface{}{
					"type": "doc", "version": 1,
					"content": []map[string]interface{}{
						{"type": "paragraph", "content": []map[string]interface{}{{"type": "text", "text": "Repo: https://github.com/example/repo"}}},
					},
				},
			},
		})
	})

	mux.HandleFunc("/rest/api/3/issue/FAIL-TRANSITION-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "transition failed", http.StatusInternalServerError)
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	tmpDir, _ := os.MkdirTemp("", "workflow-err-test")
	defer os.RemoveAll(tmpDir)

	testCases := []struct {
		name          string
		ticketID      string
		cfg           SessionConfig
		expectedError string
	}{
		{
			name:          "Ticket Fetch Fails",
			ticketID:      "BAD-1",
			cfg:           SessionConfig{IsMock: true},
			expectedError: "failed to fetch ticket with status: 404",
		},
		{
			name:     "Ticket is Blocked",
			ticketID: "BLOCKED-1",
			cfg:      SessionConfig{IsMock: true},
		},
		{
			name:          "No Repo Found",
			ticketID:      "NO-REPO-1",
			cfg:           SessionConfig{IsMock: true},
			expectedError: "no repo url found",
		},
		{
			name:     "Transition Fails (Graceful)",
			ticketID: "FAIL-TRANSITION-1",
			cfg:      SessionConfig{IsMock: true, ProjectPath: tmpDir, Cleanup: false},
			// No error expected, as transition failure is logged as a warning
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			err := ProcessJiraTicket(ctx, tc.ticketID, jClient, tc.cfg, nil)
			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Mock Session Manager
type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) StartSession(name string, command []string, cwd string) (*runner.SessionState, error) {
	args := m.Called(name, command, cwd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*runner.SessionState), args.Error(1)
}

func TestRunWorkflow_Detached_Error(t *testing.T) {
	mockSM := new(MockSessionManager)

	t.Run("StartSession fails", func(t *testing.T) {
		mockSM.On("StartSession", "test", mock.Anything, mock.Anything).Return(nil, errors.New("failed to start")).Once()
		cfg := SessionConfig{Detached: true, SessionName: "test", SessionManager: mockSM}
		err := RunWorkflow(context.Background(), cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start detached session")
		mockSM.AssertExpectations(t)
	})
}
