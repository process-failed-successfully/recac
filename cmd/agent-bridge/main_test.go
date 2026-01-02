package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun_Blocker(t *testing.T) {
	// Setup temp DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	// 1. Set Blocker
	args := []string{"agent-bridge", "blocker", "Something is wrong"}
	if err := run(args, dbPath); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Ideally we check DB state, but 'run' just prints to stdout/stderr.
	// We trust SetSignal is covered by db tests. Here we test the CLI wiring.
}

func TestRun_QA(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	args := []string{"agent-bridge", "qa"}
	if err := run(args, dbPath); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestRun_Signal(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	args := []string{"agent-bridge", "signal", "MY_KEY", "MY_VALUE"}
	if err := run(args, dbPath); err != nil {
		t.Fatalf("run failed: %v", err)
	}
}

func TestRun_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")

	// No args
	if err := run([]string{"agent-bridge"}, dbPath); err == nil {
		t.Error("Expected error for no args")
	}

	// Unknown command
	if err := run([]string{"agent-bridge", "unknown"}, dbPath); err == nil {
		t.Error("Expected error for unknown command")
	}
}
