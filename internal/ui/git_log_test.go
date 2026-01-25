package ui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestGitLogModel_Update(t *testing.T) {
	commits := []CommitItem{
		{Hash: "123", Author: "Alice", Message: "Init"},
		{Hash: "456", Author: "Bob", Message: "Fix"},
	}

	diffCalled := false
	explainCalled := false
	auditCalled := false

	fetchDiff := func(h string) (string, error) {
		diffCalled = true
		if h != "123" {
			t.Errorf("expected hash 123, got %s", h)
		}
		return "diff content", nil
	}

	explain := func(h string) (string, error) {
		explainCalled = true
		return "explanation", nil
	}

	audit := func(h string) (string, error) {
		auditCalled = true
		return "audit report", nil
	}

	m := NewGitLogModel(commits, fetchDiff, explain, audit)

	// Send WindowSizeMsg to initialize viewport dimensions
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = newM.(GitLogModel)

	// 1. Initial State
	if m.viewingDetails {
		t.Error("should not be viewing details initially")
	}

	// 2. Select item (Enter)
	// list selection is at index 0 by default ("123")
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(GitLogModel)

	// Trigger the command returned
	if cmd == nil {
		t.Fatal("expected command from Enter")
	}
	// Run the command to get the msg
	msg := cmd()

	// Since we use tea.Batch, we might get a BatchMsg
	var dMsg diffMsg
	var found bool

	if batchMsg, ok := msg.(tea.BatchMsg); ok {
		for _, subCmd := range batchMsg {
			subMsg := subCmd()
			if dm, ok := subMsg.(diffMsg); ok {
				dMsg = dm
				found = true
				break
			}
		}
	} else if dm, ok := msg.(diffMsg); ok {
		// In case it wasn't batched (though my code batches)
		dMsg = dm
		found = true
	}

	if !found {
		t.Fatalf("expected diffMsg in command result, got %T", msg)
	}
	if dMsg.content != "diff content" {
		t.Errorf("expected 'diff content', got %s", dMsg.content)
	}
	if !diffCalled {
		t.Error("fetchDiff was not called")
	}

	// Apply the diffMsg to update model state
	newM, _ = m.Update(dMsg)
	m = newM.(GitLogModel)

	if !m.viewingDetails {
		t.Error("should be viewing details after diffMsg")
	}
	if m.viewport.View() == "" {
		t.Error("viewport should have content")
	}

	// 3. Go back (Esc)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(GitLogModel)
	if m.viewingDetails {
		t.Error("should not be viewing details after Esc")
	}

	// 4. Explain (e)
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = newM.(GitLogModel)

	if cmd == nil {
		t.Fatal("expected command from 'e'")
	}
	msg = cmd()

	var aMsg analysisResultMsg
	found = false

	if batchMsg, ok := msg.(tea.BatchMsg); ok {
		for _, subCmd := range batchMsg {
			subMsg := subCmd()
			if am, ok := subMsg.(analysisResultMsg); ok {
				aMsg = am
				found = true
				break
			}
		}
	} else if am, ok := msg.(analysisResultMsg); ok {
		aMsg = am
		found = true
	}

	if !found {
		t.Fatalf("expected analysisResultMsg, got %T", msg)
	}
	if aMsg.result != "explanation" {
		t.Errorf("expected 'explanation', got %s", aMsg.result)
	}
	if !explainCalled {
		t.Error("explainFunc was not called")
	}

	// Apply analysisMsg
	newM, _ = m.Update(aMsg)
	m = newM.(GitLogModel)
	if !m.viewingDetails {
		t.Error("should be viewing details after analysisMsg")
	}

	// Back again
	m.viewingDetails = false

	// 5. Audit (s)
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = newM.(GitLogModel)

	if cmd == nil {
		t.Fatal("expected command from 's'")
	}
	msg = cmd()

	found = false
	if batchMsg, ok := msg.(tea.BatchMsg); ok {
		for _, subCmd := range batchMsg {
			subMsg := subCmd()
			if am, ok := subMsg.(analysisResultMsg); ok {
				aMsg = am
				found = true
				break
			}
		}
	} else if am, ok := msg.(analysisResultMsg); ok {
		aMsg = am
		found = true
	}

	if !found {
		t.Fatalf("expected analysisResultMsg, got %T", msg)
	}
	if aMsg.result != "audit report" {
		t.Errorf("expected 'audit report', got %s", aMsg.result)
	}
	if !auditCalled {
		t.Error("auditFunc was not called")
	}
}

func TestGitLogModel_Update_Error(t *testing.T) {
	// Test error handling
	commits := []CommitItem{{Hash: "123"}}
	fetchDiff := func(h string) (string, error) {
		return "", errors.New("git error")
	}

	m := NewGitLogModel(commits, fetchDiff, nil, nil)

	// Simulate diff fetch
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(GitLogModel)

	msg := cmd()
	var dMsg diffMsg
	if batchMsg, ok := msg.(tea.BatchMsg); ok {
		for _, subCmd := range batchMsg {
			subMsg := subCmd()
			if dm, ok := subMsg.(diffMsg); ok {
				dMsg = dm
				break
			}
		}
	} else if dm, ok := msg.(diffMsg); ok {
		dMsg = dm
	}

	newM, _ = m.Update(dMsg)
	m = newM.(GitLogModel)

	if m.viewingDetails {
		t.Error("should not switch to view mode on error")
	}
	if m.statusMessage != "Error fetching diff: git error" {
		t.Errorf("unexpected status message: %s", m.statusMessage)
	}
}
