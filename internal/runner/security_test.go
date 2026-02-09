package runner

import (
	"context"
	"os"
	"strings"
	"testing"
	"recac/internal/telemetry"
	"recac/internal/agent"
)

// MockAgentForSecurity is a simple mock that does nothing
type MockAgentForSecurity struct {
	agent.Agent
}

func TestSecurity_EnvLeak_LocalExecution(t *testing.T) {
	// 1. Setup sensitive environment variable
	secretKey := "CRITICAL_SECRET_KEY"
	secretValue := "super_secret_value_123"
	os.Setenv(secretKey, secretValue)
	defer os.Unsetenv(secretKey)

	// 2. Initialize Session with UseLocalAgent = true
	tmpDir := t.TempDir()
	session := &Session{
		Workspace:     tmpDir,
		UseLocalAgent: true,
		Project:       "security-test",
		Logger:        telemetry.NewLogger(true, "", false),
	}

	// 3. Execute "env" command
	// We need to simulate executeCommandBlock directly or via ProcessResponse
	// executor.go has executeCommandBlock as a method of Session.
	// We can use ProcessResponse with a bash block.

	response := "```bash\nenv\n```"
	output, err := session.ProcessResponse(context.Background(), response)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	// 4. Check for leak
	if strings.Contains(output, secretValue) {
		t.Errorf("CRITICAL SECURITY VULNERABILITY: Environment variable leaked in command output!\nFound: %s=%s", secretKey, secretValue)
	} else {
		t.Log("Secure: Secret not found in output.")
	}
}
