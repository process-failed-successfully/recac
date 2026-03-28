package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"recac/internal/orchestrator"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_Blockers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1/blockers" {
			w.WriteHeader(http.StatusOK)
			jobs := []orchestrator.JobInfo{
				{ID: "BLOCKER-1", Summary: "blocking job", Status: "Running"},
			}
			json.NewEncoder(w).Encode(jobs)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)

	// Test fetch cmd
	cmd := fetchBlockersCmd(server.URL, "JOB-1")
	msg := cmd()
	bm, ok := msg.(blockersMsg)
	assert.True(t, ok)
	assert.NoError(t, bm.Err)
	assert.Equal(t, 1, len(bm.Jobs))
	assert.Equal(t, "BLOCKER-1", bm.Jobs[0].ID)

	// Update model with blockersMsg
	newModel, _ := m.Update(bm)
	dm, ok := newModel.(DashboardModel)
	assert.True(t, ok)

	// Assert viewState is updated
	assert.Equal(t, viewBlockers, dm.viewState)

	// Test render contains stats
	view := dm.View()
	assert.Contains(t, view, "Blockers of JOB-1")
	assert.Contains(t, view, "BLOCKER-1")
	assert.Contains(t, view, "blocking job")
}

func TestDashboardModel_Dependents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/JOB-1/dependents" {
			w.WriteHeader(http.StatusOK)
			jobs := []orchestrator.JobInfo{
				{ID: "DEP-1", Summary: "dependent job", Status: "Pending"},
			}
			json.NewEncoder(w).Encode(jobs)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	m := NewDashboardModel(server.URL)

	// Test fetch cmd
	cmd := fetchDependentsCmd(server.URL, "JOB-1")
	msg := cmd()
	dmMsg, ok := msg.(dependentsMsg)
	assert.True(t, ok)
	assert.NoError(t, dmMsg.Err)
	assert.Equal(t, 1, len(dmMsg.Jobs))
	assert.Equal(t, "DEP-1", dmMsg.Jobs[0].ID)

	// Update model with dependentsMsg
	newModel, _ := m.Update(dmMsg)
	dm, ok := newModel.(DashboardModel)
	assert.True(t, ok)

	// Assert viewState is updated
	assert.Equal(t, viewDependents, dm.viewState)

	// Test render contains stats
	view := dm.View()
	assert.Contains(t, view, "Dependents of JOB-1")
	assert.Contains(t, view, "DEP-1")
	assert.Contains(t, view, "dependent job")
}
