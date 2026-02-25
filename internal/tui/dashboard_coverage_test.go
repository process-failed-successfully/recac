package tui

import (
	"errors"
	"io"
	"strings"
	"testing"

	"recac/internal/orchestrator"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDashboardModel_Update_Details(t *testing.T) {
	m := NewDashboardModel("http://test")

	// 1. detailsMsg Success
	job := orchestrator.JobInfo{ID: "job-1", Summary: "Details"}
	detailsMsgSuccess := detailsMsg{Job: job}
	newM, _ := m.Update(detailsMsgSuccess)
	dm := newM.(DashboardModel)

	if dm.viewState != viewDetails {
		t.Errorf("Expected viewState viewDetails, got %v", dm.viewState)
	}
	if dm.details.ID != "job-1" {
		t.Errorf("Expected details for job-1, got %s", dm.details.ID)
	}

	// 2. detailsMsg Error
	err := errors.New("failed details")
	detailsMsgError := detailsMsg{Err: err}
	newM, _ = m.Update(detailsMsgError)
	dm = newM.(DashboardModel)

	if dm.err != err {
		t.Errorf("Expected error %v, got %v", err, dm.err)
	}
}

func TestDashboardModel_Update_Logs(t *testing.T) {
	m := NewDashboardModel("http://test")

	// 1. logStreamMsg Success
	stream := io.NopCloser(strings.NewReader("log content"))
	logMsgSuccess := logStreamMsg{Stream: stream}
	newM, _ := m.Update(logMsgSuccess)
	dm := newM.(DashboardModel)

	if dm.viewState != viewLogs {
		t.Errorf("Expected viewState viewLogs, got %v", dm.viewState)
	}
	if dm.logStream == nil {
		t.Errorf("Expected logStream to be set")
	}

	// 2. logChunkMsg
	chunkMsg := logChunkMsg{Chunk: "chunk1", Err: nil}
	newM, _ = dm.Update(chunkMsg) // Use dm which has logStream set
	dm = newM.(DashboardModel)

	if dm.logs != "chunk1" {
		t.Errorf("Expected logs 'chunk1', got '%s'", dm.logs)
	}

	// 3. logChunkMsg Error
	err := errors.New("read error")
	chunkMsgError := logChunkMsg{Err: err}
	newM, _ = dm.Update(chunkMsgError)
	dm = newM.(DashboardModel)

	if dm.err != err {
		t.Errorf("Expected error %v, got %v", err, dm.err)
	}
	if dm.logStream != nil {
		t.Errorf("Expected logStream to be closed/nil on error")
	}

	// 4. logFinishedMsg
	finMsg := logFinishedMsg{Err: nil}
	newM, _ = m.Update(finMsg) // Use original m
	dm = newM.(DashboardModel)
	if dm.logStream != nil {
		t.Errorf("Expected logStream to be nil")
	}
}

func TestDashboardModel_Update_Action(t *testing.T) {
	m := NewDashboardModel("http://test")

	// actionMsg Error
	err := errors.New("action failed")
	actionMsgError := actionMsg{Err: err}
	newM, _ := m.Update(actionMsgError)
	dm := newM.(DashboardModel)

	if dm.err != err {
		t.Errorf("Expected error %v, got %v", err, dm.err)
	}
}

func TestDashboardModel_Update_Keys(t *testing.T) {
	m := NewDashboardModel("http://test")
	// Setup jobs for selection
	m.jobs = []orchestrator.JobInfo{{ID: "job-1"}}
	// Update table to reflect jobs
	m.updateTableContent()
	// Select first row (index 0)
	m.table.SetCursor(0)

	// 1. Press "c" (Cancel)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	dm := newM.(DashboardModel)

	if dm.viewState != viewConfirmation {
		t.Errorf("Expected viewState viewConfirmation, got %v", dm.viewState)
	}
	if dm.pendingAction != "cancel" {
		t.Errorf("Expected pendingAction cancel, got %s", dm.pendingAction)
	}
	if dm.pendingJobId != "job-1" {
		t.Errorf("Expected pendingJobId job-1, got %s", dm.pendingJobId)
	}

	// 2. Confirm "y"
	newM, cmd := dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	dm = newM.(DashboardModel)

	if dm.viewState != viewMain {
		t.Errorf("Expected viewState viewMain after confirmation, got %v", dm.viewState)
	}
	if dm.pendingAction != "" {
		t.Errorf("Expected pendingAction cleared")
	}
	if cmd == nil {
		t.Errorf("Expected command returned for cancel action")
	}
}

func TestDashboardModel_Update_Keys_Retry(t *testing.T) {
	m := NewDashboardModel("http://test")
	m.jobs = []orchestrator.JobInfo{{ID: "job-1"}}
	m.updateTableContent()
	m.table.SetCursor(0)

	// Press "r" (Retry)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	dm := newM.(DashboardModel)

	if dm.viewState != viewConfirmation {
		t.Errorf("Expected viewState viewConfirmation")
	}
	if dm.pendingAction != "retry" {
		t.Errorf("Expected pendingAction retry")
	}

	// Reject "n"
	newM, _ = dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	dm = newM.(DashboardModel)

	if dm.viewState != viewMain {
		t.Errorf("Expected viewState viewMain after reject")
	}
	if dm.pendingAction != "" {
		t.Errorf("Expected pendingAction cleared")
	}
}
