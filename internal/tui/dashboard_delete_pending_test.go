package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDeletePendingBulkCmd(t *testing.T) {
	tests := []struct {
		name        string
		filterType  string
		filterValue string
		statusCode  int
		response    string
		wantMsg     string
		wantErr     bool
	}{
		{
			name:        "Delete by group success",
			filterType:  "group",
			filterValue: "group-1",
			statusCode:  http.StatusOK,
			response:    `{"deleted": 5}`,
			wantMsg:     "Deleted 5 pending jobs by group",
			wantErr:     false,
		},
		{
			name:        "Delete by tag success",
			filterType:  "tag",
			filterValue: "bug",
			statusCode:  http.StatusOK,
			response:    `{"deleted": 2}`,
			wantMsg:     "Deleted 2 pending jobs by tag",
			wantErr:     false,
		},
		{
			name:        "Delete by match success",
			filterType:  "match",
			filterValue: "^job-.*$",
			statusCode:  http.StatusOK,
			response:    `{"deleted": 0}`,
			wantMsg:     "Deleted 0 pending jobs by match",
			wantErr:     false,
		},
		{
			name:        "Delete pending API error",
			filterType:  "group",
			filterValue: "group-error",
			statusCode:  http.StatusInternalServerError,
			response:    `internal server error`,
			wantMsg:     "",
			wantErr:     true,
		},
		{
			name:        "Invalid JSON response format",
			filterType:  "tag",
			filterValue: "invalid",
			statusCode:  http.StatusOK,
			response:    `{"deleted": "not-a-number"}`,
			wantMsg:     "",
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodDelete, r.Method)
				assert.Equal(t, "/jobs/pending", r.URL.Path)
				assert.Equal(t, tc.filterValue, r.URL.Query().Get(tc.filterType))

				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.response))
			}))
			defer server.Close()

			cmd := deletePendingBulkCmd(server.URL, tc.filterType, tc.filterValue)
			msg := cmd()

			actionMsg, ok := msg.(actionMsg)
			assert.True(t, ok)

			if tc.wantErr {
				assert.Error(t, actionMsg.Err)
			} else {
				assert.NoError(t, actionMsg.Err)
				assert.Equal(t, tc.wantMsg, actionMsg.Message)
			}
		})
	}
}

func TestDeletePendingBulkInput_Update(t *testing.T) {
	m := NewDashboardModel("http://localhost:8080")

	// Group
	m.viewState = viewDeletePendingGroupInput
	m.deletePendingGroupInput.SetValue("my-group")
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter")})
	// Wait, bubbletea enter key is handled differently if we use string "enter"
	// The tea.KeyMsg{Type: tea.KeyEnter} is parsed as msg.String() == "enter"
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd)
	updatedModel, ok := newModel.(DashboardModel)
	assert.True(t, ok)
	assert.Equal(t, viewMain, updatedModel.viewState)

	// Test Esc key
	m.viewState = viewDeletePendingGroupInput
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updatedModel, _ = newModel.(DashboardModel)
	assert.Equal(t, viewMain, updatedModel.viewState)

	// Tag
	m.viewState = viewDeletePendingTagInput
	m.deletePendingTagInput.SetValue("my-tag")
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd)
	updatedModel, _ = newModel.(DashboardModel)
	assert.Equal(t, viewMain, updatedModel.viewState)

	// Match
	m.viewState = viewDeletePendingMatchInput
	m.deletePendingMatchInput.SetValue("my-match")
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd)
	updatedModel, _ = newModel.(DashboardModel)
	assert.Equal(t, viewMain, updatedModel.viewState)
}
