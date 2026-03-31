package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExportJobsToJUnitXML(t *testing.T) {
	now := time.Now()
	msg := "timeout exceeded"
	jobs := []JobInfo{
		{
			ID:        "JOB-1",
			Status:    "Completed",
			Summary:   "Success Job",
			StartTime: now,
			EndTime:   now.Add(10 * time.Second),
		},
		{
			ID:            "JOB-2",
			Status:        "Failed",
			Summary:       "Failed Job",
			StatusMessage: &msg,
			StartTime:     now,
			EndTime:       now.Add(5 * time.Second),
		},
	}

	xmlStr, err := ExportJobsToJUnitXML(jobs)
	assert.NoError(t, err)

	assert.Contains(t, xmlStr, `<?xml version="1.0" encoding="UTF-8"?>`)
	assert.Contains(t, xmlStr, `<testsuites>`)
	assert.Contains(t, xmlStr, `name="RECAC Jobs"`)
	assert.Contains(t, xmlStr, `tests="2"`)
	assert.Contains(t, xmlStr, `name="JOB-1"`)
	assert.Contains(t, xmlStr, `name="JOB-2"`)
	assert.Contains(t, xmlStr, `<failure message="timeout exceeded" type="Failed">Job execution failed or was canceled.</failure>`)
}
