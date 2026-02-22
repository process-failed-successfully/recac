package main

import (
	"strings"
	"testing"
)

func TestDoctorCommand(t *testing.T) {
	// Smoke test for the doctor command wiring.
	// Detailed logic verification is done in internal/cmdutils/doctor_test.go

	t.Run("Runs successfully", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "doctor")
		// We expect no error from the command execution itself (it shouldn't panic or fail strictly)
		// even if checks fail.
		if err != nil {
			t.Logf("Command returned error: %v", err)
		}

		if !strings.Contains(output, "RECAC Doctor") {
			t.Errorf("Expected output to contain header, got: %s", output)
		}
	})

	t.Run("JSON Output", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "doctor", "--json")
		if err != nil {
			t.Logf("Command returned error: %v", err)
		}

		trimmed := strings.TrimSpace(output)
		// Should be a JSON array
		if !strings.HasPrefix(trimmed, "[") {
			t.Errorf("Expected JSON output to start with '[', got: %s", trimmed)
		}
	})
}
