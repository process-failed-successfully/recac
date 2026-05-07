package tui

import (
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestRenderDetailsExtra(t *testing.T) {
	start := time.Now().Add(-1 * time.Hour)
	end := start.Add(10 * time.Minute)

	jobs := []orchestrator.JobInfo{
		{
			ID: "JOB-1", Summary: "test1", Status: "Running", StartTime: start,
		},
		{
			ID: "JOB-2", Summary: "test2", Status: "Done", StartTime: start, EndTime: end,
		},
		{
			ID: "JOB-3", Summary: "test3", Status: "Failed", Error: "some err",
			WorkItem: orchestrator.WorkItem{Priority: 5, Tags: []string{"tag1", "tag2"}},
		},
	}

	for _, job := range jobs {
		view := renderDetails(job)
		assert.Contains(t, view, job.ID)
		assert.Contains(t, view, job.Summary)
		assert.Contains(t, view, job.Status)
		if job.Error != "" {
			assert.Contains(t, view, job.Error)
		}
	}
}

func TestRenderDetailsExtra_MetricsEnvVarsAndProgress(t *testing.T) {
	start := time.Now().Add(-1 * time.Hour)
    progress := 50
    statusMsg := "working on it"

	job := orchestrator.JobInfo{
        ID: "JOB-4", Summary: "test4", Status: "Running", StartTime: start,
        Progress: &progress,
        StatusMessage: &statusMsg,
        WorkItem: orchestrator.WorkItem{
            EnvVars: map[string]string{"ENV_VAR1": "value1"},
        },
        Metrics: map[string]float64{"metric1": 1.23},
    }

    view := renderDetails(job)
    assert.Contains(t, view, "50%")
    assert.Contains(t, view, "working on it")
    assert.Contains(t, view, "ENV_VAR1")
    assert.Contains(t, view, "value1")
    assert.Contains(t, view, "metric1")
    assert.Contains(t, view, "1.23")
}
