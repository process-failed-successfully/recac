package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSession_Start_InitScript_Security_LocalMode(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "app_spec.txt")
	os.WriteFile(specPath, []byte("test spec"), 0644)

	// Create a malicious init.sh that creates a file
	markerFile := filepath.Join(tmpDir, "pwned.txt")
	initPath := filepath.Join(tmpDir, "init.sh")
	// Use explicit path to touch to be sure
	scriptContent := "#!/bin/sh\ntouch " + markerFile + "\n"
	if err := os.WriteFile(initPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create init.sh: %v", err)
	}

	// Create session with NO Docker client -> Local Mode
	// MockAgent is defined in session_test.go in the same package
	session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

    // Explicitly set UseLocalAgent to true (NewSession does this if docker is nil, but to be sure)
    session.UseLocalAgent = true

	// Ensure we are NOT in K8s (mock env var just in case, though usually empty in tests)
	t.Setenv("KUBERNETES_SERVICE_HOST", "")

	// Start session
	// This will trigger runInitScript async
	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait a bit because init.sh runs async in a goroutine
	// The original code uses a 10m timeout context, but the script itself is fast.
	time.Sleep(200 * time.Millisecond)

	// Check if marker file exists
	// IF IT EXISTS, VULNERABILITY IS CONFIRMED.
	// For the purpose of the test, we want to FAIL if the file exists (after we fix it).
	// But right now, we expect it to exist to prove the bug.
	// I will write the test to assert that it DOES NOT exist, so it fails now.
	if _, err := os.Stat(markerFile); err == nil {
		t.Errorf("SECURITY VULNERABILITY: init.sh was executed in Local Mode! Marker file created at %s", markerFile)
	}
}
