package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"recac/internal/cmdutils"
	"recac/internal/jira"
	"recac/internal/orchestrator"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runTaskCommand directly executes RunE of the command, bypassing root execution
func runTaskCommand(t *testing.T, cmd *cobra.Command, args []string, flags map[string]string) string {
	// Set Context
	cmd.SetContext(context.Background())

	// Reset flags first
	cmd.Flags().VisitAll(func(f *pflag.Flag) { f.Value.Set(f.DefValue) })

	// Set flags
	for k, v := range flags {
		err := cmd.Flags().Set(k, v)
		require.NoError(t, err)
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute RunE directly
	err := cmd.RunE(cmd, args)

	w.Close()
	os.Stdout = oldStdout

	var output bytes.Buffer
	io.Copy(&output, r)

	require.NoError(t, err)
	return output.String()
}

func TestTaskCmd_FilePoller(t *testing.T) {
	// Setup temp file
	tmpFile, err := os.CreateTemp("", "work_items_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	viper.Reset()
	viper.Set("orchestrator.poller", "file")
	viper.Set("orchestrator.work_file", tmpFile.Name())

	// 1. Add Task
	out := runTaskCommand(t, taskAddCmd, []string{}, map[string]string{
		"summary":     "Test Task",
		"description": "Desc",
		"repo-url":    "http://repo",
		"priority":    "High",
	})
	assert.Contains(t, out, "Added task to")

	// Verify file content
	content, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)
	var items []orchestrator.WorkItem
	err = json.Unmarshal(content, &items)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Test Task", items[0].Summary)
	assert.Equal(t, "Desc", items[0].Description)
	assert.Equal(t, "High", items[0].EnvVars["PRIORITY"])

	// 2. List Tasks
	out = runTaskCommand(t, taskListCmd, []string{}, nil)
	assert.Contains(t, out, "Test Task")
	assert.Contains(t, out, items[0].ID)
}

func TestTaskCmd_FileDirPoller(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "work_items_dir_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	viper.Reset()
	viper.Set("orchestrator.poller", "file-dir")
	viper.Set("orchestrator.watch_dir", tmpDir)

	// 1. Add Task
	out := runTaskCommand(t, taskAddCmd, []string{}, map[string]string{
		"summary":  "Dir Task",
		"repo-url": "http://repo",
		"priority": "Low",
	})
	assert.Contains(t, out, "Added task to")

	// Verify file creation
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// Verify content
	content, err := os.ReadFile(filepath.Join(tmpDir, entries[0].Name()))
	require.NoError(t, err)
	var item orchestrator.WorkItem
	err = json.Unmarshal(content, &item)
	require.NoError(t, err)
	assert.Equal(t, "Dir Task", item.Summary)
	assert.Equal(t, "Low", item.EnvVars["PRIORITY"])

	// 2. List Tasks
	out = runTaskCommand(t, taskListCmd, []string{}, nil)
	assert.Contains(t, out, "Dir Task")
	assert.Contains(t, out, item.ID)
}

func TestTaskCmd_JiraPoller(t *testing.T) {
	// Mock Jira Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock Create Ticket
		if r.Method == "POST" && r.URL.Path == "/rest/api/3/issue" {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			fields := body["fields"].(map[string]interface{})
			summary := fields["summary"].(string)

			if summary == "Jira Task" {
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]string{"key": "TEST-1", "id": "10001"})
				return
			}
		}

		// Mock Search
		if r.Method == "GET" && r.URL.Path == "/rest/api/3/search/jql" {
			w.WriteHeader(http.StatusOK)
			// Return empty or one issue
			resp := map[string]interface{}{
				"issues": []map[string]interface{}{
					{
						"key": "TEST-1",
						"fields": map[string]interface{}{
							"summary": "Jira Task",
							"description": map[string]interface{}{
								"text": "Repo: http://repo.git",
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
	defer ts.Close()

	// Override GetJiraClient
	originalGetJiraClient := cmdutils.GetJiraClient
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return jira.NewClient(ts.URL, "user", "token"), nil
	}
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()

	viper.Reset()
	viper.Set("orchestrator.poller", "jira")

	// 1. Add Task (requires project)
	os.Setenv("JIRA_PROJECT_KEY", "TEST")
	defer os.Unsetenv("JIRA_PROJECT_KEY")

	out := runTaskCommand(t, taskAddCmd, []string{}, map[string]string{
		"summary":  "Jira Task",
		"repo-url": "http://repo",
	})
	assert.Contains(t, out, "Created Jira ticket: TEST-1")

	// 2. List Tasks
	out = runTaskCommand(t, taskListCmd, []string{}, nil)
	assert.Contains(t, out, "TEST-1")
	assert.Contains(t, out, "Jira Task")
}
