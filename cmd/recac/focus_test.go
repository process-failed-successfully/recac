package main

import (
	"bytes"
	"errors"
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
	os.Exit(1) // Fails!
}

func TestFocusModel_Update(t *testing.T) {
	m := initialFocusModel(10*time.Second, "Test Task")

	cmd := m.Init()
	assert.NotNil(t, cmd)

	assert.False(t, m.finished)

	newM, cmd := m.Update(timer.TimeoutMsg{ID: 0})
	fm := newM.(focusModel)
	assert.True(t, fm.finished)
	assert.Equal(t, tea.Quit(), cmd())

	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assert.Equal(t, tea.Quit(), cmd())

	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, tea.Quit(), cmd())

	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC, Runes: []rune{'c'}})
	assert.Equal(t, tea.Quit(), cmd())

	newM, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 100})

    viewStr := m.View()
    assert.Contains(t, viewStr, "Test Task")

    fm.finished = true
    viewStrDone := fm.View()
    assert.Contains(t, viewStrDone, "Done!")
}

func TestStartMusic(t *testing.T) {
	oldExec := execCommand
	execCommand = fakeFocusExecCommand
	defer func() { execCommand = oldExec }()

	err := startMusic()
    if runtime.GOOS == "darwin" || runtime.GOOS == "linux" || runtime.GOOS == "windows" {
	    assert.NoError(t, err)
    } else {
        assert.Error(t, err)
    }
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

    cmd, _, _ := newRootCmd()
	err := runFocus(cmd, []string{})
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

    cmd, _, _ := newRootCmd()
	err := runFocus(cmd, []string{})
	assert.NoError(t, err)
}

func TestFocus_RunFocus_Failures(t *testing.T) {
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
	execCommand = fakeFocusExecCommandFail
	defer func() { execCommand = oldExec }()

	oldTUI := startFocusTUIFunc
	startFocusTUIFunc = func(m tea.Model) error {
		return errors.New("tui error")
	}
	defer func() { startFocusTUIFunc = oldTUI }()

    cmd, _, _ := newRootCmd()
    var errBuf bytes.Buffer
    cmd.SetErr(&errBuf)

	err := runFocus(cmd, []string{})
	assert.Error(t, err)

}

func TestFocus_RunFocus_NoTaskPrompt(t *testing.T) {
	oldTask := focusTask
	defer func() {
		focusTask = oldTask
	}()

	focusTask = ""
    focusMusic = false
    focusDND = false

	oldTUI := startFocusTUIFunc
	startFocusTUIFunc = func(m tea.Model) error {
		return nil
	}
	defer func() { startFocusTUIFunc = oldTUI }()

    oldStdin := os.Stdin
    defer func() { os.Stdin = oldStdin }()

    r, w, _ := os.Pipe()
    w.WriteString("MyTask\n")
    w.Close()

    os.Stdin = r

    cmd, _, _ := newRootCmd()
	err := runFocus(cmd, []string{})
	assert.NoError(t, err)
    assert.Equal(t, "MyTask", focusTask)
}
