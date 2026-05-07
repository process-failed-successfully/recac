package tui

import (
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestRenderJobTable(t *testing.T) {
	// Add some jobs
	jobs := []orchestrator.JobInfo{
		{ID: "JOB-1", Status: "Running", Summary: "test1", WorkItem: orchestrator.WorkItem{Priority: 1}},
		{ID: "JOB-2", Status: "Pending", Summary: "test2", WorkItem: orchestrator.WorkItem{Tags: []string{"test"}}},
		{ID: "JOB-3", Status: "Failed", Summary: "test3"},
	}

	view := renderJobTable(jobs, "My Test Table")
	assert.Contains(t, view, "My Test Table")
	assert.Contains(t, view, "JOB-1")
	assert.Contains(t, view, "JOB-2")
	assert.Contains(t, view, "JOB-3")
	assert.Contains(t, view, "Running")
}

func TestRenderJobTable_Empty(t *testing.T) {
	view := renderJobTable(nil, "Empty Table")
	assert.Contains(t, view, "No jobs found")
}

func TestRenderJobTable_WithDates(t *testing.T) {
	start := time.Now().Add(-1 * time.Hour)
	jobs := []orchestrator.JobInfo{
		{ID: "JOB-1", Status: "Running", Summary: "test1", StartTime: start},
		{ID: "JOB-2", Status: "Done", Summary: "test2", StartTime: start, EndTime: start.Add(10 * time.Minute)},
	}

	view := renderJobTable(jobs, "Date Table")
	assert.Contains(t, view, "Date Table")
	assert.Contains(t, view, "JOB-1")
	assert.Contains(t, view, "JOB-2")
	assert.Contains(t, view, "10m0s") // exact duration for done
}
