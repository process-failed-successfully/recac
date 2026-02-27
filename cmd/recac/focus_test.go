package main

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/timer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// Mocking execCommand
var mockFocusCmdArgs []string
var mockFocusCmdName string

func fakeFocusExecCommand(command string, args ...string) *exec.Cmd {
	mockFocusCmdName = command
	mockFocusCmdArgs = args

	// Return a dummy command that succeeds (echo)
	cs := []string{"-test.run=TestFocusHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestFocusHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

func TestFocusModel_Update(t *testing.T) {
	m := initialFocusModel(10*time.Second, "Test Task")

	// Test Init
	cmd := m.Init()
	assert.NotNil(t, cmd)

	// Test Tick (simulated)
	assert.False(t, m.finished)

	// Test Timeout
	newM, cmd := m.Update(timer.TimeoutMsg{ID: 0})
	fm := newM.(focusModel)
	assert.True(t, fm.finished)
	assert.Equal(t, tea.Quit(), cmd())

	// Test Key Quit
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.Quit(), cmd())
}

func TestStartMusic(t *testing.T) {
	// Swap execCommand
	oldExec := execCommand
	execCommand = fakeFocusExecCommand
	defer func() { execCommand = oldExec }()

	err := startMusic()
	assert.NoError(t, err)

	// Verify command based on OS
	switch runtime.GOOS {
	case "darwin":
		assert.Equal(t, "open", mockFocusCmdName)
		assert.Contains(t, mockFocusCmdArgs[0], "youtube.com")
	case "linux":
		assert.Equal(t, "xdg-open", mockFocusCmdName)
	case "windows":
		assert.Equal(t, "cmd", mockFocusCmdName)
	}
}

func TestToggleDND(t *testing.T) {
	// Swap execCommand
	oldExec := execCommand
	execCommand = fakeFocusExecCommand
	defer func() { execCommand = oldExec }()

	err := toggleDND(true)

	if runtime.GOOS == "darwin" {
		// It tries 'shortcuts' first
		assert.NoError(t, err)
		assert.Equal(t, "shortcuts", mockFocusCmdName)
		assert.Equal(t, "run", mockFocusCmdArgs[0])
		assert.Contains(t, mockFocusCmdArgs[1], "Do Not Disturb")
	} else {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "only supported on macOS")
	}
}

func TestFocusFlags(t *testing.T) {
	// Reset flags
	focusDuration = 25 * time.Minute
	focusTask = ""
	focusMusic = false
	focusDND = false

	args := []string{"--duration", "10m", "--task", "My Task", "--music", "--dnd"}
	focusCmd.SetArgs(args)

	dFlag := focusCmd.Flags().Lookup("duration")
	assert.NotNil(t, dFlag)

	tFlag := focusCmd.Flags().Lookup("task")
	assert.NotNil(t, tFlag)

	mFlag := focusCmd.Flags().Lookup("music")
	assert.NotNil(t, mFlag)

	dndFlag := focusCmd.Flags().Lookup("dnd")
	assert.NotNil(t, dndFlag)
}

func TestFocus_RunFocus(t *testing.T) {
	// Reset flags
	oldDuration := focusDuration
	oldTask := focusTask
	oldMusic := focusMusic
	oldDND := focusDND

	defer func() {
		focusDuration = oldDuration
		focusTask = oldTask
		focusMusic = oldMusic
		focusDND = oldDND
	}()

	focusTask = "A specific task"
	focusMusic = false
	focusDND = false

	// Override execCommand to avoid running actual stuff
	oldExec := execCommand
	execCommand = fakeFocusExecCommand
	defer func() { execCommand = oldExec }()

	// Override TUI func
	oldTUI := startFocusTUIFunc
	startFocusTUIFunc = func(m tea.Model) error {
		return nil
	}
	defer func() { startFocusTUIFunc = oldTUI }()

	err := runFocus(focusCmd, []string{})
	assert.NoError(t, err)
}

func TestFocus_RunFocus_Music_DND(t *testing.T) {
	// Reset flags
	oldTask := focusTask
	oldMusic := focusMusic
	oldDND := focusDND

	defer func() {
		focusTask = oldTask
		focusMusic = oldMusic
		focusDND = oldDND
	}()

	focusTask = "A specific task"
	focusMusic = true
	// Testing DND logic depends on macOS via runtime.GOOS, so let's mock the exec and set the flag
	focusDND = true

	// Override execCommand to avoid running actual stuff
	oldExec := execCommand
	execCommand = fakeFocusExecCommand
	defer func() { execCommand = oldExec }()

	// Override TUI func
	oldTUI := startFocusTUIFunc
	startFocusTUIFunc = func(m tea.Model) error {
		return nil
	}
	defer func() { startFocusTUIFunc = oldTUI }()

	err := runFocus(focusCmd, []string{})
	assert.NoError(t, err)
}
