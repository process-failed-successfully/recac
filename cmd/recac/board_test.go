package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/jira"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestBoardModel_Init(t *testing.T) {
	// Setup Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Expect search request
		if r.URL.Path == "/rest/api/3/search/jql" {
			w.WriteHeader(http.StatusOK)
			// Return some issues
			resp := map[string]interface{}{
				"issues": []map[string]interface{}{
					{
						"key": "PROJ-1",
						"fields": map[string]interface{}{
							"summary": "Test Issue 1",
							"status": map[string]interface{}{
								"name": "To Do",
							},
						},
					},
					{
						"key": "PROJ-2",
						"fields": map[string]interface{}{
							"summary": "Test Issue 2",
							"status": map[string]interface{}{
								"name": "In Progress",
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Mock GetJiraClient
	origGetClient := cmdutils.GetJiraClient
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(server.URL, "user", "token"), nil
	}
	defer func() { cmdutils.GetJiraClient = origGetClient }()

	// Test Init
	client, _ := cmdutils.GetJiraClient(context.Background())
	m := initialBoardModel(client, "PROJ")

	cmd := m.Init()
	assert.NotNil(t, cmd)

	// Execute the command to get the Msg
	msg := cmd()
	issuesMsg, ok := msg.(issuesMsg)
	assert.True(t, ok)
	assert.Len(t, issuesMsg.tasks, 4) // 2 from active query, 2 from done query (mock server returns same for both calls)

	// Since mock returns same response for both queries:
	// Query 1 (!= Done): PROJ-1 (To Do), PROJ-2 (In Progress)
	// Query 2 (= Done): PROJ-1 (forced to Done), PROJ-2 (forced to Done) -> Wait, logic?

	// My mock returns "To Do" and "In Progress" status names.
	// Logic:
	// Bucket 1 (Active):
	//   PROJ-1 -> Status "To Do" -> todo
	//   PROJ-2 -> Status "In Progress" -> inProgress
	// Bucket 2 (Done):
	//   PROJ-1 -> Status "To Do" but forced to done? No, fetchIssuesCmd calls process(..., done)
	//   So it forces status to `done`.

	// So we expect:
	// Task 1: PROJ-1 (Todo)
	// Task 2: PROJ-2 (In Progress)
	// Task 3: PROJ-1 (Done)
	// Task 4: PROJ-2 (Done)

	// Verify Task 1
	assert.Equal(t, "PROJ-1", issuesMsg.tasks[0].id)
	assert.Equal(t, todo, issuesMsg.tasks[0].status)

	// Verify Task 3
	assert.Equal(t, "PROJ-1", issuesMsg.tasks[2].id)
	assert.Equal(t, done, issuesMsg.tasks[2].status)
}

func TestBoardModel_Update(t *testing.T) {
	m := BoardModel{
		cols: []column{
			newColumn(todo),
			newColumn(inProgress),
			newColumn(done),
		},
		focused: 0,
	}

	// 1. Test Navigation
	// Right
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	bm := newM.(BoardModel)
	assert.Equal(t, 1, bm.focused)

	// Right again
	newM, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	bm = newM.(BoardModel)
	assert.Equal(t, 2, bm.focused)

	// Right bound
	newM, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	bm = newM.(BoardModel)
	assert.Equal(t, 2, bm.focused)

	// Left
	newM, _ = bm.Update(tea.KeyMsg{Type: tea.KeyLeft})
	bm = newM.(BoardModel)
	assert.Equal(t, 1, bm.focused)

	// 2. Test Loading Issues
	tasks := []Task{
		{id: "1", title: "T1", status: todo},
		{id: "2", title: "T2", status: inProgress},
	}
	newM, _ = m.Update(issuesMsg{tasks: tasks})
	bm = newM.(BoardModel)

	assert.True(t, bm.loaded)
	assert.Equal(t, 1, len(bm.cols[0].list.Items())) // T1
	assert.Equal(t, 1, len(bm.cols[1].list.Items())) // T2
	assert.Equal(t, 0, len(bm.cols[2].list.Items()))
}

func TestBoardModel_MoveItem(t *testing.T) {
	// Mock Server for transition
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/PROJ-1/transitions" {
			// Get transitions request
			if r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"transitions": []map[string]interface{}{
						{"id": "11", "name": "In Progress"},
						{"id": "21", "name": "Done"},
					},
				})
				return
			}
			// Post transition request
			if r.Method == "POST" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := jira.NewClient(server.URL, "u", "p")

	// Setup model with one item in To Do
	m := BoardModel{
		client: client,
		cols: []column{
			newColumn(todo),
			newColumn(inProgress),
		},
		focused: 0,
	}

	// Add item manually
	task := Task{id: "PROJ-1", title: "Test", status: todo}
	m.cols[0].list.SetItems([]list.Item{task})

	// Trigger Move (Enter)
	// We call moveItemCmd directly or via Update
	// Update with Enter key
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.NotNil(t, cmd)

	// Execute command
	msg := cmd()

	// Should return moveMsg (success) or errorMsg
	switch v := msg.(type) {
	case errorMsg:
		t.Fatalf("Move failed: %v", v.err)
	case moveMsg:
		// Success
	default:
		t.Fatalf("Unexpected message type: %T", v)
	}
}

func TestKeyMap(t *testing.T) {
	// Verify keys are bound correctly
	assert.Equal(t, []string{"up", "k"}, keys.Up.Keys())
	assert.Equal(t, []string{"q", "ctrl+c"}, keys.Quit.Keys())
}

func TestBoardView(t *testing.T) {
	m := BoardModel{
		cols: []column{
			newColumn(todo),
			newColumn(inProgress),
			newColumn(done),
		},
		focused: 0,
		loaded:  true,
	}

	// Set window size so view renders
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	// Add some tasks
	m.cols[0].list.SetItems([]list.Item{
		Task{id: "T-1", title: "Task One", status: todo},
	})
	m.cols[1].list.SetItems([]list.Item{
		Task{id: "T-2", title: "Task Two", status: inProgress},
	})

	output := m.View()

	assert.NotEmpty(t, output)
	assert.Contains(t, output, "To Do")
	assert.Contains(t, output, "In Progress")
	assert.Contains(t, output, "Task One")
	assert.Contains(t, output, "Task Two")
}
