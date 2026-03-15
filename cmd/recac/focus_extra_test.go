package main

import (
	"runtime"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToggleDND_Platform(t *testing.T) {
	// Swap execCommand
	oldExec := execCommand
	execCommand = fakeFocusExecCommand
	defer func() { execCommand = oldExec }()

	err := toggleDND(true)

	if runtime.GOOS == "darwin" {
		// It tries 'shortcuts' first
		// In test, execCommand succeeds
		assert.NoError(t, err)
		assert.Equal(t, "shortcuts", mockFocusCmdName)
		assert.Equal(t, "run", mockFocusCmdArgs[0])
		// Wait, mockFocusCmdArgs might be empty if we didn't call it?
		// We called toggleDND(true) -> "Turn On Do Not Disturb"
		if len(mockFocusCmdArgs) > 1 {
			assert.Contains(t, mockFocusCmdArgs[1], "Do Not Disturb")
		}
	} else {
		// Non-macOS should error
		assert.Error(t, err)
		// We might not hit the exec command if it fails early
	}
}

func TestFocusRun_Input(t *testing.T) {
	// We can't easily test Scanln directly without replacing os.Stdin,
	// which is messy in parallel tests.
	// But we can check flags logic if we set them.

	// Reset flags
	oldDuration := focusDuration
	oldTask := focusTask
	defer func() {
		focusDuration = oldDuration
		focusTask = oldTask
	}()

	focusDuration = 10 * time.Minute
	focusTask = "My Task"

	// If task is set, it shouldn't ask for input.
	// But running TUI will block.
	// So we can only test side effects or small pieces.

	// Let's verify initial model logic
	m := initialFocusModel(focusDuration, focusTask)
	assert.Equal(t, "My Task", m.task)
	assert.False(t, m.finished)

	// View check
	view := m.View()
	assert.Contains(t, view, "My Task")
	assert.Contains(t, view, "Focus:")
}

func TestStartMusicOS_Extra(t *testing.T) {
	originalExec := execCommand
	defer func() { execCommand = originalExec }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}

	err := startMusic()
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		assert.NoError(t, err)
	} else {
		assert.Error(t, err)
	}
}
