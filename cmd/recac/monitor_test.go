package main

import (
	"fmt"
	"testing"
)

func TestMonitorCmd(t *testing.T) {
	// Backup original startDashboard
	originalStartDashboard := startDashboard
	defer func() {
		startDashboard = originalStartDashboard
	}()

	expectedHost := "http://test-host:8080"
	called := false

	// Mock startDashboard
	startDashboard = func(host string) error {
		called = true
		if host != expectedHost {
			return fmt.Errorf("expected host %s, got %s", expectedHost, host)
		}
		return nil
	}

	// Use executeCommand helper which handles flag resetting and execution
	_, err := executeCommand(rootCmd, "monitor", "--host", expectedHost)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !called {
		t.Error("Expected startDashboard to be called, but it wasn't")
	}
}
