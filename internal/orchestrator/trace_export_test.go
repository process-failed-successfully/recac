package orchestrator

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportTraceToJSON_Success(t *testing.T) {
	now := time.Now()
	jobs := []JobInfo{
		{
			ID:        "job-1",
			Summary:   "Build App",
			Status:    "Completed",
			StartTime: now.Add(-30 * time.Minute),
			EndTime:   now.Add(-20 * time.Minute),
			WorkItem: WorkItem{
				AgentProvider: "openai",
				AgentModel:    "gpt-4",
			},
		},
		{
			ID:        "job-2",
			Summary:   "Test App",
			Status:    "Failed",
			StartTime: now.Add(-10 * time.Minute),
			EndTime:   now.Add(-5 * time.Minute),
			Error:     "test failed",
		},
		{
			ID:        "job-3",
			Summary:   "Pending Job",
			Status:    "Pending",
			StartTime: time.Time{}, // Not started
		},
	}

	jsonData, err := ExportTraceToJSON(jobs)
	require.NoError(t, err)

	var events []TraceEvent
	err = json.Unmarshal(jsonData, &events)
	require.NoError(t, err)

	// Should only be 2 events since job-3 hasn't started
	assert.Len(t, events, 2)

	// Verify job-1
	assert.Equal(t, "job-1", events[0].Name)
	assert.Equal(t, "job", events[0].Cat)
	assert.Equal(t, "X", events[0].Ph)
	assert.Equal(t, jobs[0].StartTime.UnixMicro(), events[0].Ts)
	dur1 := jobs[0].EndTime.Sub(jobs[0].StartTime).Microseconds()
	assert.Equal(t, dur1, events[0].Dur)
	assert.Equal(t, 1, events[0].Pid)
	assert.Equal(t, 1, events[0].Tid)
	assert.Equal(t, "good", events[0].Cname)
	assert.Equal(t, "Build App", events[0].Args["summary"])
	assert.Equal(t, "Completed", events[0].Args["status"])
	assert.Equal(t, "openai", events[0].Args["provider"])
	assert.Equal(t, "gpt-4", events[0].Args["model"])

	// Verify job-2
	assert.Equal(t, "job-2", events[1].Name)
	assert.Equal(t, "terrible", events[1].Cname)
	assert.Equal(t, "test failed", events[1].Args["error"])
}

func TestExportTraceToJSON_Empty(t *testing.T) {
	jsonData, err := ExportTraceToJSON([]JobInfo{})
	require.NoError(t, err)
	assert.Equal(t, "null", string(jsonData))
}
