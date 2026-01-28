package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"recac/internal/jira"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestTaskSubmit_File(t *testing.T) {
	tempDir := t.TempDir()
	workFile := filepath.Join(tempDir, "work_items.json")
	viper.Set("orchestrator.poller", "file")
	viper.Set("orchestrator.work_file", workFile)

	// Reset flags
	taskDescription = "Test Description"
	taskPriority = "High"
	taskLabels = []string{"test-label"}
	taskPoller = "file" // Override to be sure

	cmd := taskSubmitCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runTaskSubmit(cmd, []string{"Test Task"})
	assert.NoError(t, err)

	// Verify file content
	content, err := os.ReadFile(workFile)
	assert.NoError(t, err)
	var tasks []LocalTask
	err = json.Unmarshal(content, &tasks)
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "Test Task", tasks[0].Title)
	assert.Equal(t, "Test Description", tasks[0].Description)
	assert.Equal(t, "High", tasks[0].Priority)
	assert.Equal(t, "pending", tasks[0].Status)
}

func TestTaskSubmit_FileDir(t *testing.T) {
	tempDir := t.TempDir()
	viper.Set("orchestrator.poller", "file-dir")
	viper.Set("orchestrator.watch_dir", tempDir)

	taskDescription = "Dir Description"
	taskPoller = "file-dir"

	cmd := taskSubmitCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runTaskSubmit(cmd, []string{"Dir Task"})
	assert.NoError(t, err)

	// Verify file created
	files, err := os.ReadDir(tempDir)
	assert.NoError(t, err)
	assert.Len(t, files, 1)

	content, err := os.ReadFile(filepath.Join(tempDir, files[0].Name()))
	assert.NoError(t, err)
	var task LocalTask
	err = json.Unmarshal(content, &task)
	assert.NoError(t, err)
	assert.Equal(t, "Dir Task", task.Title)
}

func TestTaskSubmit_Jira(t *testing.T) {
	// Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/project" {
			// Mock GetFirstProjectKey
			w.Write([]byte(`[{"key": "TEST"}]`))
			return
		}
		if r.URL.Path == "/rest/api/3/issue" && r.Method == "POST" {
			// Mock CreateTicket
			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)
			fields := payload["fields"].(map[string]interface{})
			summary := fields["summary"].(string)

			// Assertions could be done here or just assume success
			if summary == "Jira Task" {
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{"key": "TEST-101", "self": "http://jira/issue/101"}`))
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Mock Factory
	origFactory := getJiraClientFunc
	defer func() { getJiraClientFunc = origFactory }()
	getJiraClientFunc = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(ts.URL, "user", "token"), nil
	}

	viper.Set("orchestrator.poller", "jira")
	viper.Set("jira.project_key", "TEST")
	viper.Set("jira.url", ts.URL)

	taskDescription = "Jira Description"
	taskPoller = "jira"
	taskPriority = "" // Reset
	taskLabels = nil

	cmd := taskSubmitCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runTaskSubmit(cmd, []string{"Jira Task"})
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "TEST-101")
}

func TestTaskList_File(t *testing.T) {
	tempDir := t.TempDir()
	workFile := filepath.Join(tempDir, "work_items.json")
	viper.Set("orchestrator.poller", "file")
	viper.Set("orchestrator.work_file", workFile)

	// Create dummy file
	tasks := []LocalTask{
		{ID: "T1", Title: "Task 1", Status: "pending"},
		{ID: "T2", Title: "Task 2", Status: "done"},
	}
	data, _ := json.Marshal(tasks)
	os.WriteFile(workFile, data, 0644)

	taskPoller = "file"
	cmd := taskListCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runTaskList(cmd, nil)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Task 1")
	assert.Contains(t, out.String(), "Task 2")
}

func TestTaskList_FileDir(t *testing.T) {
	tempDir := t.TempDir()
	viper.Set("orchestrator.poller", "file-dir")
	viper.Set("orchestrator.watch_dir", tempDir)

	// Create dummy file
	task := LocalTask{ID: "TD1", Title: "Dir Task 1", Status: "pending"}
	data, _ := json.Marshal(task)
	os.WriteFile(filepath.Join(tempDir, "task-1.json"), data, 0644)

	taskPoller = "file-dir"
	cmd := taskListCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runTaskList(cmd, nil)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Dir Task 1")
}

func TestTaskList_Jira(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" {
			// Check if JQL contains correct label
			jql := r.URL.Query().Get("jql")
			if jql != "labels = \"recac-agent\"" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Write([]byte(`{
				"issues": [
					{
						"key": "TEST-101",
						"fields": {
							"summary": "Jira Task 1",
							"status": {"name": "To Do"}
						}
					}
				]
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Mock Factory
	origFactory := getJiraClientFunc
	defer func() { getJiraClientFunc = origFactory }()
	getJiraClientFunc = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(ts.URL, "user", "token"), nil
	}

	viper.Set("orchestrator.poller", "jira")
	viper.Set("orchestrator.jira_label", "recac-agent")
	viper.Set("jira.url", ts.URL)

	taskPoller = "jira"
	cmd := taskListCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := runTaskList(cmd, nil)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "TEST-101")
	assert.Contains(t, out.String(), "Jira Task 1")
}
