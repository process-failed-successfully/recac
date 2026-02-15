package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"recac/internal/agent"
	"recac/internal/cmdutils"
	"recac/internal/git"
	"recac/internal/jira"
	"github.com/stretchr/testify/assert"
)

func TestProcessDirectTask_Table(t *testing.T) {
	// Backup original functions
	originalSetup := cmdutils.SetupWorkspace
	originalRunWorkflow := RunWorkflow
	defer func() {
		cmdutils.SetupWorkspace = originalSetup
		RunWorkflow = originalRunWorkflow
	}()

	tests := []struct {
		name           string
		setupErr       error
		runWorkflowErr error
		expectedErr    string
	}{
		{
			name:        "SetupWorkspace Failure",
			setupErr:    errors.New("setup failed"),
			expectedErr: "setup failed",
		},
		{
			name:           "RunWorkflow Failure",
			runWorkflowErr: errors.New("workflow failed"),
			expectedErr:    "workflow failed",
		},
		{
			name: "Success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock SetupWorkspace
			cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
				if tt.setupErr != nil {
					return "", tt.setupErr
				}
				os.MkdirAll(workspace, 0755)
				return repoURL, nil
			}

			// Mock RunWorkflow
			RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
				return tt.runWorkflowErr
			}

			tmpDir, _ := os.MkdirTemp("", "workflow-direct-table")
			defer os.RemoveAll(tmpDir)

			cfg := SessionConfig{
				ProjectPath: tmpDir,
				RepoURL:     "https://github.com/example/direct",
				Summary:     "Do something",
				IsMock:      true,
			}

			err := ProcessDirectTask(context.Background(), cfg)

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProcessJiraTicket_Extra(t *testing.T) {
	// Backup original functions
	originalSetup := cmdutils.SetupWorkspace
	originalRunWorkflow := RunWorkflow
	defer func() {
		cmdutils.SetupWorkspace = originalSetup
		RunWorkflow = originalRunWorkflow
	}()

	// Mock SetupWorkspace globally for this test suite
	cmdutils.SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
		os.MkdirAll(workspace, 0755)
		return repoURL, nil
	}

	// Mock RunWorkflow globally
	RunWorkflow = func(ctx context.Context, cfg SessionConfig) error {
		return nil
	}

	tests := []struct {
		name            string
		ticketID        string
		ticketSummary   string
		description     string
		blockers        []string // IDs of blocking tickets
		ignoredBlockers map[string]bool
		expectSkip      bool // If skipped due to blockers (returns nil)
		expectedErr     string
		mockTransitions bool // Whether to mock transitions endpoint
		epicParent      string
	}{
		{
			name:            "Blocked Ticket but Ignored",
			ticketID:        "TEST-IGNORED",
			ticketSummary:   "Ignored Blocker Ticket",
			blockers:        []string{"BLOCKER-1"},
			ignoredBlockers: map[string]bool{"BLOCKER-1": true},
			description:     "Repo: https://github.com/example/repo",
			mockTransitions: true,
		},
		{
			name:            "Epic Detection",
			ticketID:        "TEST-EPIC",
			ticketSummary:   "Epic Child Ticket",
			description:     "Repo: https://github.com/example/repo",
			epicParent:      "EPIC-123",
			mockTransitions: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create Mock Server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Handle Ticket Fetch
				if r.Method == "GET" && r.URL.Path == fmt.Sprintf("/rest/api/3/issue/%s", tt.ticketID) {
					// Construct response
					fields := map[string]interface{}{
						"summary": tt.ticketSummary,
						"description": map[string]interface{}{
							"type": "doc",
							"version": 1,
							"content": []interface{}{
								map[string]interface{}{
									"type": "paragraph",
									"content": []interface{}{
										map[string]interface{}{
											"type": "text",
											"text": tt.description,
										},
									},
								},
							},
						},
					}

					if tt.epicParent != "" {
						fields["parent"] = map[string]interface{}{
							"key": tt.epicParent,
						}
					}

					// Construct Issuelinks for blockers
					var links []interface{}
					for _, b := range tt.blockers {
						links = append(links, map[string]interface{}{
							"type": map[string]interface{}{
								"inward": "is blocked by",
							},
							"inwardIssue": map[string]interface{}{
								"key": b,
								"fields": map[string]interface{}{
									"status": map[string]interface{}{
										"name": "Open",
									},
								},
							},
						})
					}
					fields["issuelinks"] = links

					json.NewEncoder(w).Encode(map[string]interface{}{
						"key":    tt.ticketID,
						"fields": fields,
					})
					return
				}

				// Handle Transitions
				if tt.mockTransitions && r.URL.Path == fmt.Sprintf("/rest/api/3/issue/%s/transitions", tt.ticketID) {
					if r.Method == "GET" {
						json.NewEncoder(w).Encode(map[string]interface{}{
							"transitions": []interface{}{
								map[string]interface{}{"id": "11", "name": "In Progress"},
							},
						})
						return
					} else if r.Method == "POST" {
						w.WriteHeader(http.StatusNoContent)
						return
					}
				}

				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			client := jira.NewClient(server.URL, "user", "token")

			tmpDir, _ := os.MkdirTemp("", "workflow-jira-table")
			defer os.RemoveAll(tmpDir)

			cfg := SessionConfig{
				ProjectPath: tmpDir,
				IsMock:      true,
				Cleanup:     true,
			}

			err := ProcessJiraTicket(context.Background(), tt.ticketID, client, cfg, tt.ignoredBlockers)

			if tt.expectSkip {
				// Should return nil, but log "SKIPPING"
				assert.NoError(t, err)
			} else if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestRunWorkflow_Errors tests error paths in RunWorkflow
func TestRunWorkflow_Errors(t *testing.T) {
	// Mock cmdutils.GetAgentClient to fail
	originalGetAgentClient := cmdutils.GetAgentClient
	defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()

	// Case 1: Agent Client Failure
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, errors.New("agent init failed")
	}

	tmpDir, _ := os.MkdirTemp("", "workflow-run-errors")
	defer os.RemoveAll(tmpDir)

	cfg := SessionConfig{
		ProjectPath: tmpDir,
		SessionName: "run-error-test",
		IsMock:      false, // Trigger normal flow
		AllowDirty:  true,
	}

	err := RunWorkflow(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent init failed")
}
