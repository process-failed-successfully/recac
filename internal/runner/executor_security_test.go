package runner

import (
	"context"
	"log/slog"
	"recac/internal/agent"
	"recac/internal/notify"
	"recac/internal/security"
	"strings"
	"testing"
)

func TestProcessResponse_Security(t *testing.T) {
	mockDocker := &MockDockerForExec{}
	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      slog.Default(),
		Scanner:     security.NewRegexScanner(), // Use real scanner
	}

	// 1. Dangerous Command
	resp := "I will delete everything.\n```bash\nrm -rf /\n```"
	out, err := s.ProcessResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	// Verify it was blocked
	if !strings.Contains(out, "[BLOCKED] Command 1 blocked by security scanner") {
		t.Errorf("Expected blocked message, got: %s", out)
	}
	if !strings.Contains(out, "Root Deletion") { // Description from scanner
		t.Errorf("Expected description 'Root Deletion', got: %s", out)
	}

	// Verify it was NOT executed
	for _, executed := range mockDocker.ExecutedCmds {
		if strings.Contains(executed, "rm -rf /") {
			t.Errorf("Dangerous command was executed! %s", executed)
		}
	}

	// 2. Safe Command (Sanity Check)
	respSafe := "I will list files.\n```bash\nls -la\n```"
	outSafe, err := s.ProcessResponse(context.Background(), respSafe)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	if strings.Contains(outSafe, "[BLOCKED]") {
		t.Errorf("Safe command was blocked! %s", outSafe)
	}

	// Verify it WAS executed
	found := false
	for _, executed := range mockDocker.ExecutedCmds {
		if strings.Contains(executed, "ls -la") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Safe command was NOT executed")
	}

	// 3. Commented Dangerous Command (False Positive Check)
	// This ensures that dangerous commands inside comments are ignored.
	respComment := "I am explaining rm -rf /\n```bash\n# Do not run rm .ssh keys\necho 'safe'\n```"
	outComment, err := s.ProcessResponse(context.Background(), respComment)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	if strings.Contains(outComment, "[BLOCKED]") {
		t.Errorf("Commented dangerous command was blocked! %s", outComment)
	}

	// 4. Inline Comment (False Positive Check)
	// This ensures that dangerous commands inside inline comments are ignored.
	respInline := "Inline comment.\n```bash\necho 'safe' # rm -rf /\n```"
	outInline, err := s.ProcessResponse(context.Background(), respInline)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if strings.Contains(outInline, "[BLOCKED]") {
		t.Errorf("Inline comment was blocked! %s", outInline)
	}

	// 5. Bypass Attempt (Security Check)
	// This ensures that dangerous commands followed by other commands are still blocked.
	respBypass := "Trying to bypass.\n```bash\nrm -rf /\necho 'done'\n```"
	outBypass, err := s.ProcessResponse(context.Background(), respBypass)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if !strings.Contains(outBypass, "[BLOCKED]") {
		t.Errorf("Bypass attempt was NOT blocked! %s", outBypass)
	}

	// 6. Exploit Attempt (Quote Hijacking)
	// This ensures that comments inside quotes are NOT masked, and dangerous commands after them are detected.
	respExploit := "Exploit attempt.\n```bash\necho \"This is a # trick\"; rm -rf /\n```"
	outExploit, err := s.ProcessResponse(context.Background(), respExploit)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if !strings.Contains(outExploit, "[BLOCKED]") {
		t.Errorf("Exploit attempt was NOT blocked! %s", outExploit)
	}

	// 7. Parameter Expansion Exploit (Bash Syntax Check)
	// This ensures that # inside parameter expansion (e.g., ${VAR#pattern}) is NOT treated as a comment start.
	respParam := "Parameter expansion.\n```bash\necho ${VAR#pattern}; rm -rf /\n```"
	outParam, err := s.ProcessResponse(context.Background(), respParam)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if !strings.Contains(outParam, "[BLOCKED]") {
		t.Errorf("Parameter expansion exploit was NOT blocked! %s", outParam)
	}

	// 8. Background Process Bypass Attempt
	// This ensures that using & separator doesn't bypass the check
	respBg := "Background process.\n```bash\nrm -rf /&\n```"
	outBg, err := s.ProcessResponse(context.Background(), respBg)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}
	if !strings.Contains(outBg, "[BLOCKED]") {
		t.Errorf("Background process bypass was NOT blocked! %s", outBg)
	}
}

// TestProcessResponse_MockAgentSafe ensures that the MockAgent's standard response
// for the prime-python scenario is NOT blocked by the security scanner.
// This is critical for CI smoke tests.
func TestProcessResponse_MockAgentSafe(t *testing.T) {
	mockDocker := &MockDockerForExec{}
	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      slog.Default(),
		Scanner:     security.NewRegexScanner(),
	}

	mockAgent := agent.NewMockAgent()
	ctx := context.Background()

	// Simulate the exact prompt sent in the E2E test
	// The prompt usually contains "Implement the solution... ID:[PRIMES]"
	prompt := "Implement the solution for task ID:[PRIMES]. Create a python script named 'primes.py'..."

	resp, err := mockAgent.Send(ctx, prompt)
	if err != nil {
		t.Fatalf("MockAgent failed: %v", err)
	}

	// Ensure the response contains the expected bash block
	if !strings.Contains(resp, "cat << 'EOF' > primes.py") {
		t.Fatalf("MockAgent response does not contain expected script:\n%s", resp)
	}

	out, err := s.ProcessResponse(ctx, resp)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	// Verify it was NOT blocked
	if strings.Contains(out, "[BLOCKED]") {
		t.Errorf("MockAgent response was blocked by security scanner!\nOutput: %s", out)
	}

	// Verify commands were executed
	// The entire script is executed as a single bash block, plus blocker checks.
	foundScript := false
	for _, cmd := range mockDocker.ExecutedCmds {
		if strings.Contains(cmd, "cat << 'EOF' > primes.py") {
			foundScript = true
			break
		}
	}
	if !foundScript {
		t.Errorf("Expected script execution not found in commands: %v", mockDocker.ExecutedCmds)
	}
}
