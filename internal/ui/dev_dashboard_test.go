package ui

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
)

// TestHelperProcess is used to mock exec.Command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// Print something to stdout
	os.Stdout.WriteString("mock output")
	os.Exit(0)
}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestExecuteDevCommand(t *testing.T) {
	// Mock
	oldExec := DevExecCmdFactory
	DevExecCmdFactory = fakeExecCommand
	defer func() { DevExecCmdFactory = oldExec }()

	cmd := executeDevCommand("echo test")
	msg := cmd()

	finishMsg, ok := msg.(CommandFinishedMsg)
	assert.True(t, ok)
	assert.Equal(t, "mock output", finishMsg.Output)
	assert.Nil(t, finishMsg.Err)
}

func TestDevDashboardUpdate(t *testing.T) {
	model := DevDashboardModel{
		Status: "Idle",
		Ready:  true, // Simulate initialized
	}

	// Test RunCommandMsg
	updatedModel, cmd := model.Update(RunCommandMsg{})
	m := updatedModel.(DevDashboardModel)
	assert.Equal(t, "Running", m.Status)
	assert.NotNil(t, cmd)

	// Test CommandFinishedMsg
	finishMsg := CommandFinishedMsg{
		Output:   "success",
		Duration: 100 * time.Millisecond,
	}
	updatedModel, _ = model.Update(finishMsg)
	m = updatedModel.(DevDashboardModel)
	assert.Equal(t, "Success", m.Status)
	assert.Equal(t, "success", m.Output)
}

func TestDevDashboardPendingRun(t *testing.T) {
	model := DevDashboardModel{
		Status: "Running",
	}

	// Trigger run while running
	updatedModel, cmd := model.Update(RunCommandMsg{})
	m := updatedModel.(DevDashboardModel)
	assert.True(t, m.pendingRun)
	assert.Nil(t, cmd) // Should NOT return a command

	// Finish current run
	finishMsg := CommandFinishedMsg{Output: "done"}
	updatedModel, cmd = m.Update(finishMsg)
	m = updatedModel.(DevDashboardModel)

	assert.False(t, m.pendingRun)
	assert.Equal(t, "Running", m.Status)
	assert.NotNil(t, cmd) // Should trigger the pending run
}

func TestDevDashboardDebounce(t *testing.T) {
	model := DevDashboardModel{
		Debounce:   100 * time.Millisecond,
		Extensions: []string{".go"},
	}

	// Send file change
	event := fsnotify.Event{Name: "test.go", Op: fsnotify.Write}
	updatedModel, _ := model.Update(FileChangeMsg{Event: event})
	m := updatedModel.(DevDashboardModel)

	assert.Equal(t, 1, m.debounceTag)

	// Simulate tick with correct tag
	updatedModel, cmd := m.Update(DebounceMsg{tag: 1})
	m = updatedModel.(DevDashboardModel)
	assert.Equal(t, "Running", m.Status) // Should run
	assert.NotNil(t, cmd)

	// Send another file change (tag increments)
	updatedModel, _ = m.Update(FileChangeMsg{Event: event})
	m = updatedModel.(DevDashboardModel)
	assert.Equal(t, 2, m.debounceTag)

	// Simulate old tick (tag 1)
	updatedModel, cmd = m.Update(DebounceMsg{tag: 1})
	m = updatedModel.(DevDashboardModel)
	assert.Nil(t, cmd) // Should NOT run
}
