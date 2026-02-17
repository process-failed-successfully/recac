package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProcessJiraTicket_Blocked(t *testing.T) {
	// Mock SetupWorkspace to ensure it's NOT called
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		t.Error("SetupWorkspace should not be called for blocked ticket")
		return "", nil
	}

	// Mock Jira Server
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock Ticket Response
	mux.HandleFunc("/rest/api/3/issue/BLOCKED-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "BLOCKED-1",
			"fields": map[string]interface{}{
				"summary": "Blocked Ticket",
				"issuelinks": []interface{}{
					map[string]interface{}{
						"type": map[string]interface{}{
							"inward": "is blocked by",
						},
						"inwardIssue": map[string]interface{}{
							"key": "BLOCKER-1",
							"fields": map[string]interface{}{
								"status": map[string]interface{}{
									"name": "Open",
								},
							},
						},
					},
				},
			},
		})
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	cfg := SessionConfig{
		Cleanup: true,
	}

	err := ProcessJiraTicket(context.Background(), "BLOCKED-1", jClient, cfg, nil)
	assert.NoError(t, err)
}

func TestProcessJiraTicket_Epic(t *testing.T) {
	// Mock SetupWorkspace to verify Epic Key
	originalSetup := cmdutils.SetupWorkspace
	defer func() { cmdutils.SetupWorkspace = originalSetup }()

	var capturedEpicKey string
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		capturedEpicKey = epicKey
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	// Mock RunWorkflow to verify SessionConfig has epic key
	originalRunWorkflow := RunWorkflow
	defer func() { RunWorkflow = originalRunWorkflow }()
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		assert.Equal(t, "EPIC-123", cfg.JiraEpicKey)
		return nil
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/rest/api/3/issue/CHILD-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key": "CHILD-1",
			"fields": map[string]interface{}{
				"summary": "Child Ticket",
				"parent": map[string]interface{}{
					"key": "EPIC-123",
				},
				"description": map[string]interface{}{
					"type": "doc",
					"content": []interface{}{},
				},
			},
		})
	})

	// Mock Transitions (needed by ProcessJiraTicket)
	mux.HandleFunc("/rest/api/3/issue/CHILD-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]interface{}{"transitions": []interface{}{}})
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	jClient := jira.NewClient(server.URL, "user", "token")
	tmpDir := t.TempDir()

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		RepoURL: "https://github.com/test/repo", // Skip repo extraction
	}

	err := ProcessJiraTicket(context.Background(), "CHILD-1", jClient, cfg, nil)
	assert.NoError(t, err)
	assert.Equal(t, "EPIC-123", capturedEpicKey)
}

// MockSessionManagerCoverage for coverage test
type MockSessionManagerCoverage struct {
	mock.Mock
}

func (m *MockSessionManagerCoverage) StartSession(name, goal string, command []string, cwd string) (*runner.SessionState, error) {
	args := m.Called(name, goal, command, cwd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*runner.SessionState), args.Error(1)
}

func TestRunWorkflow_Detached_Coverage(t *testing.T) {
	mockSM := new(MockSessionManagerCoverage)
	tmpDir := t.TempDir()

	cfg := SessionConfig{
		Detached:       true,
		SessionName:    "detached-coverage",
		Goal:           "test goal",
		ProjectPath:    tmpDir,
		SessionManager: mockSM,
		CommandPrefix:  []string{"start"},
	}

	expectedState := &runner.SessionState{
		PID:     456,
		LogFile: "coverage.log",
	}

	mockSM.On("StartSession", "detached-coverage", "test goal", mock.Anything, tmpDir).Return(expectedState, nil)

	err := RunWorkflow(context.Background(), cfg)
	assert.NoError(t, err)
	mockSM.AssertExpectations(t)
}
