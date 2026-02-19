package tui

import (
	"errors"
	"io"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestDashboardModel_Update_LogStreamMsg(t *testing.T) {
	model := NewDashboardModel("http://test")

	// Create a pipe to simulate the stream
	r, w := io.Pipe()
	defer w.Close()

	msg := logStreamMsg{Stream: r}

	// Act
	updatedModel, cmd := model.Update(msg)
	m := updatedModel.(DashboardModel)

	// Assert
	assert.Equal(t, viewLogs, m.viewState)
	assert.NotNil(t, m.logStream)
	assert.Equal(t, "", m.logs)
	assert.NotNil(t, cmd) // Should return waitForLogChunk command
}

func TestDashboardModel_Update_LogChunkMsg(t *testing.T) {
	model := NewDashboardModel("http://test")
	model.viewState = viewLogs
	model.logs = "Previous logs\n"

	// Need a stream to be present for it to continue reading
	r, w := io.Pipe()
	defer w.Close()
	model.logStream = r

	msg := logChunkMsg{Chunk: "New chunk"}

	// Act
	updatedModel, cmd := model.Update(msg)
	m := updatedModel.(DashboardModel)

	// Assert
	assert.Equal(t, "Previous logs\nNew chunk", m.logs)
	assert.Contains(t, m.viewport.View(), "New chunk")
	assert.NotNil(t, cmd) // Should return next waitForLogChunk command
}

func TestDashboardModel_Update_LogChunkMsg_Error(t *testing.T) {
	model := NewDashboardModel("http://test")
	r, w := io.Pipe()
	defer w.Close()
	model.logStream = r

	err := errors.New("read error")
	msg := logChunkMsg{Err: err}

	// Act
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(DashboardModel)

	// Assert
	assert.Equal(t, err, m.err)
	assert.Nil(t, m.logStream) // Should be closed/nil
}

func TestDashboardModel_Update_LogChunkMsg_EOF(t *testing.T) {
	model := NewDashboardModel("http://test")
	r, w := io.Pipe()
	defer w.Close()
	model.logStream = r

	msg := logChunkMsg{Err: io.EOF}

	// Act
	updatedModel, cmd := model.Update(msg)
	m := updatedModel.(DashboardModel)

	// Assert
	assert.Nil(t, m.err) // EOF is not an error state for the model
	assert.Nil(t, m.logStream) // Should be closed/nil
	assert.Nil(t, cmd) // No more reading
}

func TestWaitForLogChunk(t *testing.T) {
	// Setup
	r, w := io.Pipe()

	go func() {
		w.Write([]byte("Hello Stream"))
		w.Close()
	}()

	cmd := waitForLogChunk(r)

	// Act
	msg := cmd()

	// Assert
	chunkMsg, ok := msg.(logChunkMsg)
	assert.True(t, ok)
	assert.Equal(t, "Hello Stream", chunkMsg.Chunk)
	assert.Nil(t, chunkMsg.Err)

	// Call again to get EOF
	cmd = waitForLogChunk(r)
	msg = cmd()
	chunkMsg, ok = msg.(logChunkMsg)
	assert.True(t, ok)
	assert.Equal(t, io.EOF, chunkMsg.Err)
}

func TestStreamJobLogs_Success(t *testing.T) {
	// This is hard to test fully without mocking HTTP client,
	// but we can test that it returns a function that returns a msg.
	cmd := streamJobLogs("http://invalid-host", "job-1")
	assert.NotNil(t, cmd)

	// Executing it will likely fail with network error, which returns logStreamMsg with Err
	msg := cmd()
	streamMsg, ok := msg.(logStreamMsg)
	assert.True(t, ok)
	assert.Error(t, streamMsg.Err)
}

func TestDashboardModel_ExitLogs_ClosesStream(t *testing.T) {
	model := NewDashboardModel("http://test")
	model.viewState = viewLogs

	r, w := io.Pipe()
	model.logStream = r

	// Act
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m := updatedModel.(DashboardModel)

	// Assert
	assert.Equal(t, viewMain, m.viewState)
	assert.Nil(t, m.logStream)

	// Verify stream is closed by writing to it - should fail
	// io.Pipe writer returns ErrClosedPipe if reader is closed
	_, err := w.Write([]byte("test"))
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}
