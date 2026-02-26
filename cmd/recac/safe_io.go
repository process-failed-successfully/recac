package main

import (
	"os"
	"recac/internal/undo"
)

// undoCaptureFunc allows mocking the backup capture process.
// Default implementation uses undo.Manager.
var undoCaptureFunc = func(paths ...string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	m := undo.NewManager(cwd)
	return m.Capture(paths...)
}

// safeWriteFile writes data to a file safely by first capturing a backup.
// It uses undoCaptureFunc to backup the file (if it exists) or record its creation.
// Then it proceeds to write the file using writeFileFunc.
func safeWriteFile(path string, data []byte, perm os.FileMode) error {
	// 1. Capture state (backup existing or record creation)
	if _, err := undoCaptureFunc(path); err != nil {
		// Just warn? Or fail? Fail is safer to ensure undo capability.
		// However, for CLI tools, maybe we shouldn't block if undo fails?
		// Let's print a warning but proceed?
		// No, the requirement is "safe" write.
		// But if we return error, the operation stops.
		// Let's return error to be safe.
		return err
	}

	// 2. Write file
	return writeFileFunc(path, data, perm)
}

// safeAppendFile appends data to a file safely by first capturing a backup.
func safeAppendFile(path string, data []byte, perm os.FileMode) error {
	// 1. Capture state
	if _, err := undoCaptureFunc(path); err != nil {
		return err
	}

	// 2. Append file
	// We use os.OpenFile directly here, or we can use a mockable func if needed.
	// For now, let's use os.OpenFile but allow mocking via package var if we want strict testing.
	// But writeFileFunc is os.WriteFile (overwrite).
	// Let's stick to os.OpenFile for now as it's standard for appending.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}
