package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSNotifyWatcher_Coverage(t *testing.T) {
	// Create a real FSNotifyWatcher
	watcher, err := NewFSNotifyWatcher()
	if err != nil {
		t.Skip("Skipping FSNotify test (inotify limit or other system issue)")
	}
	// We will close it explicitly later, but defer just in case
	defer func() { _ = watcher.Close() }()

	// Verify Events() returns a channel
	assert.NotNil(t, watcher.Events())

	// Verify Errors() returns a channel
	assert.NotNil(t, watcher.Errors())

	// Verify Add() works
	tmpDir := t.TempDir()
	err = watcher.Add(tmpDir)
	assert.NoError(t, err)

	// Verify Close() works
	err = watcher.Close()
	assert.NoError(t, err)

	// Verify AddRecursive works (at least doesn't panic)
	watcher2, err := NewFSNotifyWatcher()
	if err != nil {
		t.Skip("Skipping FSNotify test (inotify limit or other system issue)")
	}
	defer func() { _ = watcher2.Close() }()

	// Create a subdirectory to test recursion logic
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	err = watcher2.AddRecursive(tmpDir)
	assert.NoError(t, err)
}
