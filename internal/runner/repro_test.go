package runner

import (
	"context"
	"recac/internal/notify"
	"recac/internal/telemetry"
	"testing"
)

func TestRepro_SmokeTest_NoOp(t *testing.T) {
	// 1. Setup Session with Mock-like configuration
	session := &Session{
		UseLocalAgent: true,
		Docker:        nil, // Simulate nil Docker in local mode
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Logger:        telemetry.NewLogger(true, "", false),
		Project:       "repro-test",
	}

	// 2. Mock Agent Response (Exact string from internal/agent/mock.go)
	response := `I will initialize the project features.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "primes",
  "features": []
}
EOF
` + "```" + `
`

	// 3. Process Response
	output, err := session.ProcessResponse(context.Background(), response)
	if err != nil {
		t.Fatalf("ProcessResponse returned error: %v", err)
	}

	// 4. Verify commands were executed (output should contain the command execution log)
	// executeCommandBlock logs "executing command block".
	// Since we are running in unit test without real local env setup (or maybe we are?),
	// exec.Command might fail if agent-bridge is not found.
	// But ProcessResponse catches errors and returns them in output or logs them.
	// IMPORTANT: executeCommandBlock returns the output string.
	if output == "" {
		t.Error("ProcessResponse returned empty output, expected execution logs")
	}

	// We can't easily check "commands_executed" metric here without inspecting logs or internal state.
	// But if output is empty, it means no blocks matched (or execution produced no output).
	// With the mock response, the regex SHOULD match.
}

func TestRepro_Panic_NilDocker(t *testing.T) {
	// Verify checkBlockers doesn't panic when Docker is nil and UseLocalAgent is false
	session := &Session{
		UseLocalAgent: false,
		Docker:        nil,
		Notifier:      notify.NewManager(func(string, ...interface{}) {}),
		Logger:        telemetry.NewLogger(true, "", false),
	}

	// Should not panic
	err := session.checkBlockers(context.Background())
	if err != nil {
		// It might return error if no env available, but shouldn't panic
		t.Logf("checkBlockers returned error (expected/allowed): %v", err)
	}
}
