package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJiraListCmd(t *testing.T) {
	// Setup Mock Jira Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search" {
			// Basic Auth Check
			user, pass, ok := r.BasicAuth()
			if !ok || user != "user" || pass != "token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			jql := r.URL.Query().Get("jql")
			if jql == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Mock Response
			response := map[string]interface{}{
				"issues": []map[string]interface{}{
					{
						"key": "PROJ-1",
						"fields": map[string]interface{}{
							"summary": "Fix login bug",
							"status": map[string]interface{}{
								"name": "In Progress",
							},
							"assignee": map[string]interface{}{
								"displayName": "John Doe",
							},
							"updated": "2023-10-27T10:00:00.000+0000",
						},
					},
					{
						"key": "PROJ-2",
						"fields": map[string]interface{}{
							"summary": "Add new feature",
							"status": map[string]interface{}{
								"name": "To Do",
							},
							"updated": "2023-10-26T15:30:00.000+0000",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Backup original config
	origURL := viper.GetString("jira.url")
	origUser := viper.GetString("jira.username")
	origToken := viper.GetString("jira.api_token")
	defer func() {
		viper.Set("jira.url", origURL)
		viper.Set("jira.username", origUser)
		viper.Set("jira.api_token", origToken)
	}()

	// Set Config to point to Mock Server
	viper.Set("jira.url", ts.URL)
	viper.Set("jira.username", "user")
	viper.Set("jira.api_token", "token")
	// Backup environment variables
	origEnvURL := os.Getenv("JIRA_URL")
	origEnvUser := os.Getenv("JIRA_USERNAME")
	origEnvToken := os.Getenv("JIRA_API_TOKEN")
	defer func() {
		os.Setenv("JIRA_URL", origEnvURL)
		os.Setenv("JIRA_USERNAME", origEnvUser)
		os.Setenv("JIRA_API_TOKEN", origEnvToken)
	}()

	// Clear environment variables that might interfere
	os.Setenv("JIRA_URL", "")
	os.Setenv("JIRA_USERNAME", "")
	os.Setenv("JIRA_API_TOKEN", "")

	t.Run("List Table Output", func(t *testing.T) {
		buf := new(bytes.Buffer)
		jiraListCmd.SetOut(buf)
		// Ensure flags are reset
		jiraListCmd.Flags().Set("json", "false")
		jiraListCmd.Flags().Set("jql", "assignee = currentUser()")

		err := runJiraListCmd(jiraListCmd, []string{})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "KEY")
		assert.Contains(t, output, "SUMMARY")
		assert.Contains(t, output, "STATUS")
		assert.Contains(t, output, "PROJ-1")
		assert.Contains(t, output, "Fix login bug")
		assert.Contains(t, output, "In Progress")
		assert.Contains(t, output, "John Doe")
		assert.Contains(t, output, "PROJ-2")
		assert.Contains(t, output, "Add new feature")
		assert.Contains(t, output, "To Do")
		assert.Contains(t, output, "Unassigned")
		// Check date format roughly
		assert.Contains(t, output, "2023-10-27")
	})

	t.Run("List JSON Output", func(t *testing.T) {
		buf := new(bytes.Buffer)
		jiraListCmd.SetOut(buf)
		jiraListCmd.Flags().Set("json", "true")

		err := runJiraListCmd(jiraListCmd, []string{})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, `"key": "PROJ-1"`)

		var issues []map[string]interface{}
		err = json.Unmarshal(buf.Bytes(), &issues)
		require.NoError(t, err)
		assert.Len(t, issues, 2)
		assert.Equal(t, "PROJ-1", issues[0]["key"])
	})

	t.Run("Empty List", func(t *testing.T) {
		// Mock empty response
		emptyTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"issues": []map[string]interface{}{},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer emptyTS.Close()

		viper.Set("jira.url", emptyTS.URL)

		buf := new(bytes.Buffer)
		jiraListCmd.SetOut(buf)
		jiraListCmd.Flags().Set("json", "false")

		err := runJiraListCmd(jiraListCmd, []string{})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "No tickets found.")
	})
}
