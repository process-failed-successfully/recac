package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"recac/internal/cmdutils"
	"recac/internal/jira"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestRunBoard(t *testing.T) {
	// Setup Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"issues": []interface{}{},
		})
	}))
	defer server.Close()

	// Mock GetJiraClient
	origGetClient := cmdutils.GetJiraClient
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(server.URL, "user", "token"), nil
	}
	defer func() { cmdutils.GetJiraClient = origGetClient }()

	// Mock runBoardTUIFunc
	origRunBoardTUIFunc := runBoardTUIFunc
	runBoardTUIFunc = func(m BoardModel) error {
		return nil
	}
	defer func() { runBoardTUIFunc = origRunBoardTUIFunc }()

	// 1. Run with argument
	cmd := &cobra.Command{}
	err := runBoard(cmd, []string{"PROJ-KEY"})
	assert.NoError(t, err)

	// 2. Run without argument (auto-discovery)
	serverDiscovery := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Response for searching projects
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"issues": []map[string]interface{}{
				{
					"key": "PROJ-123",
				},
			},
		})
	}))
	defer serverDiscovery.Close()

	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(serverDiscovery.URL, "user", "token"), nil
	}

	err = runBoard(cmd, []string{})
	assert.NoError(t, err)
}

func TestRunBoard_Error(t *testing.T) {
	// Mock GetJiraClient failure
	origGetClient := cmdutils.GetJiraClient
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return nil, errors.New("auth failed")
	}
	defer func() { cmdutils.GetJiraClient = origGetClient }()

	cmd := &cobra.Command{}
	err := runBoard(cmd, []string{"PROJ"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auth failed")
}

func TestBoardModel_Init(t *testing.T) {
	// Setup Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" {
			w.WriteHeader(http.StatusOK)
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

	client := jira.NewClient(server.URL, "user", "token")
	m := initialBoardModel(client, "PROJ")

	cmd := m.Init()
	assert.NotNil(t, cmd)

	msg := cmd()
	issuesMsg, ok := msg.(issuesMsg)
	assert.True(t, ok)
	assert.Len(t, issuesMsg.tasks, 4) // 2 calls * 2 issues

	assert.Equal(t, "PROJ-1", issuesMsg.tasks[0].id)
	assert.Equal(t, todo, issuesMsg.tasks[0].status)
}

func TestBoardModel_Update(t *testing.T) {
	client := jira.NewClient("http://example.com", "u", "p")
	m := initialBoardModel(client, "PROJ")
	m.loaded = true

	// 1. Window Resize
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	bm := newM.(BoardModel)
	assert.Equal(t, 100, bm.width)
	assert.Equal(t, 50, bm.height)

	// 2. Navigation
	// Right -> 1
	newM, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	bm = newM.(BoardModel)
	assert.Equal(t, 1, bm.focused)
	// Right -> 2
	newM, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	bm = newM.(BoardModel)
	assert.Equal(t, 2, bm.focused)
	// Right -> max
	newM, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRight})
	bm = newM.(BoardModel)
	assert.Equal(t, 2, bm.focused)

	// Left -> 1
	newM, _ = bm.Update(tea.KeyMsg{Type: tea.KeyLeft})
	bm = newM.(BoardModel)
	assert.Equal(t, 1, bm.focused)

	// Quit
	newM, cmd := bm.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.Quit(), cmd())

	// Refresh
	newM, cmd = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	bm = newM.(BoardModel)
	assert.False(t, bm.loaded) // should set loaded false
	assert.NotNil(t, cmd)

	// issuesMsg
	tasks := []Task{{id: "1", status: todo}}
	newM, _ = bm.Update(issuesMsg{tasks: tasks})
	bm = newM.(BoardModel)
	assert.True(t, bm.loaded)
	assert.Equal(t, 1, len(bm.cols[0].list.Items()))

	// errorMsg
	err := errors.New("oops")
	newM, _ = bm.Update(errorMsg{err})
	bm = newM.(BoardModel)
	assert.Equal(t, err, bm.err)

	// moveMsg
	newM, cmd = bm.Update(moveMsg{})
	assert.NotNil(t, cmd) // should trigger fetch
}

func TestMoveItemCmd(t *testing.T) {
	// Mock Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "transitions") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == "GET" && strings.Contains(r.URL.Path, "transitions") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"transitions": []map[string]interface{}{
					{"id": "1", "name": "In Progress"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := jira.NewClient(server.URL, "u", "p")
	m := initialBoardModel(client, "PROJ")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.cols[0].list.SetItems([]list.Item{
		Task{id: "T-1", title: "Task 1", status: todo},
	})
	m.focused = 0

	// Move forward (Enter)
	cmd := moveItemCmd(m, 1)
	msg := cmd()
	_, ok := msg.(moveMsg)
	assert.True(t, ok)

	// Move backward from Todo (should fail or do nothing)
	cmd = moveItemCmd(m, -1)
	msg = cmd()
	assert.Nil(t, msg) // nil because switch returns nil
}

func TestMoveItemCmd_Error(t *testing.T) {
	// Mock Server to fail
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := jira.NewClient(server.URL, "u", "p")
	m := initialBoardModel(client, "PROJ")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.cols[0].list.SetItems([]list.Item{
		Task{id: "T-1", status: todo},
	})

	cmd := moveItemCmd(m, 1)
	msg := cmd()
	errMsg, ok := msg.(errorMsg)
	assert.True(t, ok)
	assert.Error(t, errMsg.err)
}

func TestBoardView(t *testing.T) {
	m := BoardModel{}

	// Error state
	m.err = errors.New("fail")
	assert.Contains(t, m.View(), "Error: fail")

	// Loading state
	m.err = nil
	m.loaded = false
	assert.Contains(t, m.View(), "Loading...")

	// Loaded state
	m.loaded = true
	m.cols = []column{newColumn(todo)}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40}) // Resize to render
	assert.Contains(t, m.View(), "To Do")
}

func TestTaskMethods(t *testing.T) {
	task := Task{id: "ID", title: "Title", status: todo}
	assert.Equal(t, "Title", task.FilterValue())
	assert.Equal(t, "ID", task.Title())
	assert.Equal(t, "Title", task.Description())
}
