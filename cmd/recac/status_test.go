package main

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockSessionManager is a mock implementation of the ISessionManager interface for testing.
type MockSessionManager struct {
	Sessions []*runner.SessionState
	Err      error
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Sessions, nil
}

func (m *MockSessionManager) SaveSession(session *runner.SessionState) error {
	return nil // Not needed for this test
}

func (m *MockSessionManager) LoadSession(name string) (*runner.SessionState, error) {
	return nil, nil // Not needed for this test
}

func (m *MockSessionManager) StopSession(name string) error {
	return nil // Not needed for this test
}

func (m *MockSessionManager) GetSessionLogs(name string) (string, error) {
	return "", nil // Not needed for this test
}

func TestStatusCommandOutput(t *testing.T) {
	// --- Setup ---
	sm := &MockSessionManager{
		Sessions: []*runner.SessionState{
			{
				Name:      "test-session",
				Status:    "running",
				PID:       12345,
				StartTime: time.Now(),
				LogFile:   "/tmp/test.log",
			},
		},
	}

	// --- Execute ---
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := showStatus(sm)
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// --- Verify ---
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "PID")
	assert.Contains(t, output, "UPTIME")
	assert.Contains(t, output, "LOG FILE")
	assert.Contains(t, output, "test-session")
	assert.Contains(t, output, "RUNNING")
	assert.Contains(t, output, "12345")
}

func TestStatusCommandNoSessions(t *testing.T) {
	// --- Setup ---
	sm := &MockSessionManager{
		Sessions: []*runner.SessionState{},
	}

	// --- Execute ---
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := showStatus(sm)
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// --- Verify ---
	assert.Contains(t, output, "No active or past sessions found.")
}
