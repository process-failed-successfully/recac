package orchestrator

import (
	"strings"
	"testing"
	"time"
)

func TestExportTimelineToMermaid(t *testing.T) {
	now := time.Now()

	jobs := []JobInfo{
		{
			ID:        "JOB-1",
			Summary:   "Success Job",
			Status:    "Completed",
			StartTime: now.Add(-10 * time.Minute),
			EndTime:   now.Add(-5 * time.Minute),
		},
		{
			ID:        "JOB-2",
			Summary:   "Running Job",
			Status:    "Running",
			StartTime: now.Add(-2 * time.Minute),
		},
		{
			ID:        "JOB-3",
			Summary:   "Failed Job",
			Status:    "Failed",
			StartTime: now.Add(-15 * time.Minute),
			EndTime:   now.Add(-10 * time.Minute),
		},
		{
			ID:        "JOB-4",
			Summary:   "Pending Job",
			Status:    "Pending",
			StartTime: time.Time{},
		},
	}

	mermaid := ExportTimelineToMermaid(jobs)

	if !strings.Contains(mermaid, "gantt") {
		t.Errorf("Expected output to contain 'gantt'")
	}
	if !strings.Contains(mermaid, "title Job Execution Timeline") {
		t.Errorf("Expected output to contain 'title'")
	}
	if !strings.Contains(mermaid, "JOB-1") {
		t.Errorf("Expected output to contain JOB-1")
	}
	if !strings.Contains(mermaid, "JOB-2") {
		t.Errorf("Expected output to contain JOB-2")
	}
	if !strings.Contains(mermaid, "JOB-3") {
		t.Errorf("Expected output to contain JOB-3")
	}

	// Verify section headers
	if !strings.Contains(mermaid, "section Completed") {
		t.Errorf("Expected output to contain 'section Completed'")
	}
	if !strings.Contains(mermaid, "section Active") {
		t.Errorf("Expected output to contain 'section Active'")
	}
	if !strings.Contains(mermaid, "section Failed") {
		t.Errorf("Expected output to contain 'section Failed'")
	}

	// Verify modifiers
	if !strings.Contains(mermaid, "JOB-1 :done") {
		t.Errorf("Expected JOB-1 to be marked as done")
	}
	if !strings.Contains(mermaid, "JOB-2 :active") {
		t.Errorf("Expected JOB-2 to be marked as active")
	}
	if !strings.Contains(mermaid, "JOB-3 :crit") {
		t.Errorf("Expected JOB-3 to be marked as crit")
	}
}

func TestExportTimelineToMermaid_Empty(t *testing.T) {
	mermaid := ExportTimelineToMermaid(nil)
	if mermaid != "gantt\n    title Job Execution Timeline\n" {
		t.Errorf("Unexpected output for empty jobs: %s", mermaid)
	}
}
