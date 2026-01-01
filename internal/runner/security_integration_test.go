package runner

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/docker"
	"strings"
	"testing"
)

// MockUnsafeAgent outputs a secret
type MockUnsafeAgent struct {
	Response string
}

func (m *MockUnsafeAgent) Send(ctx context.Context, prompt string) (string, error) {
	// Simulate agent leaking a key
	return m.Response, nil
}

func (m *MockUnsafeAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, nil
}

func TestSecurityIntegration_BlocksSecrets(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("Spec"), 0644)
	mockDocker, _ := docker.NewMockClient()
	mockAgent := &MockUnsafeAgent{
		Response: "Here is a secret: AKIAIOSFODNN7EXAMPLE",
	}

	// Init Session
	session := NewSession(mockDocker, mockAgent, tmpDir, "alpine", "test-project", 1)
	session.MaxIterations = 1

	// Capture output to verify blocking
	// Since we print to stdout, we can't easily capture it in a unit test without redirecting os.Stdout.
	// However, we can verify the behavior by checking if the DB has the observation.
	// If blocked, it shouldn't be saved to DB. (Our logic: Scan -> If Fail Continue -> Save DB)

	ctx := context.Background()
	// It might error with ErrNoOp because no commands, or reach max iterations.
	if err := session.RunLoop(ctx); err != nil && err != ErrMaxIterations && err != ErrNoOp && err != ErrStalled {
		t.Fatalf("RunLoop failed: %v", err)
	}

	// Verify DB does NOT contain the secret
	if session.DBStore == nil {
		t.Fatal("DBStore not initialized")
	}

	history, err := session.DBStore.QueryHistory(10)
	if err != nil {
		t.Fatalf("QueryHistory failed: %v", err)
	}

	for _, obs := range history {
		if strings.Contains(obs.Content, "AKIA") {
			t.Errorf("Security Breach! Found secret in DB: %s", obs.Content)
		}
	}

	if len(history) > 0 {
		t.Logf("DB has %d observations (good, maybe from other phases if any, but shouldn't have the secret)", len(history))
	} else {
		t.Log("DB is empty (Good, secret was blocked)")
	}
}
