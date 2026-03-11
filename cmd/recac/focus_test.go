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
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func fakeFocusExecCommandFail(command string, args ...string) *exec.Cmd {
	mockFocusCmdName = command
	mockFocusCmdArgs = args

	cs := []string{"-test.run=TestFocusHelperProcessFail", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS_FAIL=1")
	return cmd
}

func TestFocusHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

func TestFocusHelperProcessFail(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS_FAIL") != "1" {
		return
	}
	os.Exit(1)
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
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC, Runes: []rune{'c'}})
	assert.Equal(t, tea.Quit(), cmd())

    // View
    viewStr := m.View()
    assert.Contains(t, viewStr, "Test Task")

    // View Finished
    fm.finished = true
    viewStrDone := fm.View()
    assert.Contains(t, viewStrDone, "Done!")
}

func TestStartMusic(t *testing.T) {
	oldExec := execCommand
	execCommand = fakeFocusExecCommand
	defer func() { execCommand = oldExec }()

	err := startMusic()
	assert.NoError(t, err)
}

func TestToggleDND(t *testing.T) {
	oldExec := execCommand
	execCommand = fakeFocusExecCommand
	defer func() { execCommand = oldExec }()

	err := toggleDND(true)
	if runtime.GOOS == "darwin" {
		assert.NoError(t, err)
	}

	err = toggleDND(false)
	if runtime.GOOS == "darwin" {
		assert.NoError(t, err)
	}
}

func TestToggleDND_Fail(t *testing.T) {
	oldExec := execCommand
	execCommand = fakeFocusExecCommandFail
	defer func() { execCommand = oldExec }()

	err := toggleDND(true)
	if runtime.GOOS == "darwin" {
		assert.Error(t, err)
	}
}


func TestFocus_RunFocus(t *testing.T) {
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

	oldExec := execCommand
	execCommand = fakeFocusExecCommand
	defer func() { execCommand = oldExec }()

	oldTUI := startFocusTUIFunc
	startFocusTUIFunc = func(m tea.Model) error {
		return nil
	}
	defer func() { startFocusTUIFunc = oldTUI }()

	err := runFocus(focusCmd, []string{})
	assert.NoError(t, err)
}

func TestFocus_RunFocus_Music_DND(t *testing.T) {
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
	focusDND = true

	oldExec := execCommand
	execCommand = fakeFocusExecCommand
	defer func() { execCommand = oldExec }()

	oldTUI := startFocusTUIFunc
	startFocusTUIFunc = func(m tea.Model) error {
		return nil
	}
	defer func() { startFocusTUIFunc = oldTUI }()

	err := runFocus(focusCmd, []string{})
	assert.NoError(t, err)
}
