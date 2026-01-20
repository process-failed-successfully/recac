package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"recac/internal/jira"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type callbackMockSpawner struct {
	spawned  []WorkItem
	spawnErr error
	mu       sync.Mutex
	onSpawn  func(WorkItem)
}

func (m *callbackMockSpawner) Spawn(ctx context.Context, item WorkItem) error {
	m.mu.Lock()
	m.spawned = append(m.spawned, item)
	m.mu.Unlock()
	if m.onSpawn != nil {
		m.onSpawn(item)
	}
	return m.spawnErr
}

func (m *callbackMockSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	return nil
}

func TestOrchestrator_Integration_FileFlow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orch_integration_file")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	workFile := filepath.Join(tmpDir, "work.json")
	
	// Initial state: 2 work items
	items := []WorkItem{
		{ID: "TASK-1", Summary: "Summary 1", Description: "Desc 1"},
		{ID: "TASK-2", Summary: "Summary 2", Description: "Desc 2"},
	}
	data, _ := json.Marshal(items)
	err = os.WriteFile(workFile, data, 0644)
	require.NoError(t, err)

	poller := NewFilePoller(workFile)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Run orchestrator
	err = orch.Run(ctx, silentLogger)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Verify both items were spawned
	spawner.mu.Lock()
	assert.Len(t, spawner.spawned, 2)
	ids := []string{spawner.spawned[0].ID, spawner.spawned[1].ID}
	assert.Contains(t, ids, "TASK-1")
	assert.Contains(t, ids, "TASK-2")
	spawner.mu.Unlock()

	// Verify file is now empty (items claimed)
	polled, _ := poller.Poll(context.Background(), silentLogger)
	assert.Empty(t, polled)
}

func TestOrchestrator_Integration_JiraFlow(t *testing.T) {
	testCases := []struct {
		name               string
		mockIssues         []map[string]interface{}
		mockTransitions    []map[string]interface{}
		expectedSpawnCount int
		expectedClaim      bool
		spawnErr           bool
	}{
		{
			name: "Success Single Ticket",
			mockIssues: []map[string]interface{}{
				{
					"key": "JIRA-1",
					"fields": map[string]interface{}{
						"summary": "Fix bug 1",
						"description": map[string]interface{}{
							"type": "doc",
							"content": []interface{}{
								map[string]interface{}{
									"type": "paragraph",
									"content": []interface{}{
										map[string]interface{}{
											"type": "text",
											"text": "Detailed description 1. Repo: https://github.com/example/repo",
										},
									},
								},
							},
						},
					},
				},
			},
			mockTransitions: []map[string]interface{}{
				{"id": "31", "name": "In Progress"},
			},
			expectedSpawnCount: 1,
			expectedClaim:      true,
		},
		{
			name: "Spawn Failure Status Update",
			mockIssues: []map[string]interface{}{
				{
					"key": "JIRA-3",
					"fields": map[string]interface{}{
						"summary": "Failure test",
						"description": map[string]interface{}{
							"type": "doc",
							"content": []interface{}{
								map[string]interface{}{
									"type": "paragraph",
									"content": []interface{}{
										map[string]interface{}{
											"type": "text",
											"text": "Repo: https://github.com/example/fail",
										},
									},
								},
							},
						},
					},
				},
			},
			mockTransitions: []map[string]interface{}{
				{"id": "31", "name": "In Progress"},
				{"id": "99", "name": "Failed"},
			},
			expectedSpawnCount: 1, // It tries to spawn
			expectedClaim:      true,
			spawnErr:           true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			transitionsCalled := make(map[string][]string)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()

				if r.URL.Path == "/rest/api/3/search/jql" {
					var availableIssues []map[string]interface{}
					for _, issue := range tc.mockIssues {
						key := issue["key"].(string)
						// If already transitioned, assume it doesn't match JQL anymore
						if len(transitionsCalled[key]) == 0 {
							availableIssues = append(availableIssues, issue)
						}
					}
					resp := map[string]interface{}{"issues": availableIssues}
					json.NewEncoder(w).Encode(resp)
					return
				}

				if r.Method == "GET" && filepath.Base(r.URL.Path) == "transitions" {
					resp := map[string]interface{}{"transitions": tc.mockTransitions}
					json.NewEncoder(w).Encode(resp)
					return
				}

				if r.Method == "POST" && filepath.Base(r.URL.Path) == "transitions" {
					key := filepath.Base(filepath.Dir(r.URL.Path))
					var payload map[string]interface{}
					json.NewDecoder(r.Body).Decode(&payload)
					trans := payload["transition"].(map[string]interface{})
					id := trans["id"].(string)
					transitionsCalled[key] = append(transitionsCalled[key], id)
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			jClient := jira.NewClient(server.URL, "user", "token")
			poller := NewJiraPoller(jClient, "project = TEST")
			spawner := &callbackMockSpawner{
				onSpawn: func(item WorkItem) {
					// Simulate successful transition so it's "claimed"
					// We use "In Progress" ID 31 or "Failed" ID 99
					status := "In Progress"
					if tc.spawnErr {
						status = "Failed"
					}
					// Fire update in background as spawner does
					go func() {
						_ = poller.UpdateStatus(context.Background(), item, status, "")
					}()
				},
			}
			if tc.spawnErr {
				spawner.spawnErr = fmt.Errorf("spawn failed")
			}
			orch := New(poller, spawner, 50*time.Millisecond)

			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()

			_ = orch.Run(ctx, silentLogger)

			spawner.mu.Lock()
			assert.Len(t, spawner.spawned, tc.expectedSpawnCount)
			spawner.mu.Unlock()

			mu.Lock()
			if tc.expectedClaim {
				assert.Contains(t, transitionsCalled, tc.mockIssues[0]["key"])
			}
			if tc.spawnErr {
				// Should have tried to transition to "Failed"
				// Note: SmartTransition depends on name matching. 
				// In our mock we have "Failed" (id 99).
				assert.Contains(t, transitionsCalled[tc.mockIssues[0]["key"].(string)], "99")
			}
			mu.Unlock()
		})
	}
}
