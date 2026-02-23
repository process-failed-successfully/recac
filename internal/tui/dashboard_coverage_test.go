package tui

import (
	"recac/internal/orchestrator"
	"strings"
	"testing"
	"time"
)

func TestRenderDetails_FullCoverage(t *testing.T) {
	// Case 1: Job with EndTime and Error
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	job := orchestrator.JobInfo{
		ID:        "JOB-ERR",
		Summary:   "Failed Job",
		Status:    "Failed",
		StartTime: startTime,
		EndTime:   endTime,
		Error:     "Something went wrong",
		WorkItem: orchestrator.WorkItem{
			RepoURL: "http://repo",
		},
	}

	output := renderDetails(job)

	if !strings.Contains(output, "End Time") {
		t.Error("Expected output to contain 'End Time'")
	}
	if !strings.Contains(output, endTime.Format(time.RFC3339)) {
		t.Errorf("Expected output to contain formatted EndTime: %s", endTime.Format(time.RFC3339))
	}
	if !strings.Contains(output, "Duration") {
		t.Error("Expected output to contain 'Duration'")
	}
	if !strings.Contains(output, "1h") {
		t.Error("Expected output to contain duration '1h'")
	}

	if !strings.Contains(output, "Error") {
		t.Error("Expected output to contain 'Error'")
	}
	if !strings.Contains(output, "Something went wrong") {
		t.Error("Expected output to contain error message")
	}
}

func TestTick_Coverage(t *testing.T) {
	cmd := tick()
	if cmd == nil {
		t.Error("Expected tick() to return a command")
	}
}

func TestUpdateTableContent_Sorting(t *testing.T) {
	m := NewDashboardModel("host")
	now := time.Now()

	// Add jobs in random order
	m.jobs = []orchestrator.JobInfo{
		{ID: "old", StartTime: now.Add(-2 * time.Hour)},
		{ID: "new", StartTime: now},
		{ID: "mid", StartTime: now.Add(-1 * time.Hour)},
	}

	// Ensure table is initialized by NewDashboardModel
	m.updateTableContent()

	rows := m.table.Rows()
	if len(rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(rows))
	}

	// Should be sorted new -> old
	if rows[0][0] != "new" {
		t.Errorf("Expected 'new', got '%s'", rows[0][0])
	}
	if rows[1][0] != "mid" {
		t.Errorf("Expected 'mid', got '%s'", rows[1][0])
	}
	if rows[2][0] != "old" {
		t.Errorf("Expected 'old', got '%s'", rows[2][0])
	}
}

func TestUpdateTableContent_Duration(t *testing.T) {
	m := NewDashboardModel("host")
	now := time.Now()

	m.jobs = []orchestrator.JobInfo{
		{
			ID: "finished",
			StartTime: now.Add(-10 * time.Minute),
			EndTime: now.Add(-5 * time.Minute),
		},
	}

	m.updateTableContent()
	rows := m.table.Rows()

	// Duration should be 5m0s
	if !strings.Contains(rows[0][3], "5m") {
		t.Errorf("Expected duration to contain '5m', got '%s'", rows[0][3])
	}
}

func TestLimitString_Short(t *testing.T) {
	s := "hello"
	if res := limitString(s, 10); res != "hello" {
		t.Errorf("Expected 'hello', got '%s'", res)
	}
}

func TestLimitString_Long(t *testing.T) {
	s := "hello world"
	if res := limitString(s, 5); res != "hello..." {
		t.Errorf("Expected 'hello...', got '%s'", res)
	}
}


func TestDashboardModel_Update_TickMsg_Batch(t *testing.T) {
	m := NewDashboardModel("host")
	msg := tickMsg(time.Now())

	_, cmd := m.Update(msg)

	// Should return a batch command
	if cmd == nil {
		t.Error("Expected batch command from tick update")
	}
}
