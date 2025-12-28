package runner

import (
	"context"
	"recac/internal/docker"
	"strings"
	"testing"
)

// MockUnsafeAgent outputs a secret
type MockUnsafeAgent struct{}

func (m *MockUnsafeAgent) Send(ctx context.Context, prompt string) (string, error) {
	// Simulate agent leaking a key
	return "Here is the key: AKIAIOSFODNN7EXAMPLE123", nil
}

func TestSecurityIntegration_BlocksSecrets(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	mockDocker, _ := docker.NewMockClient()
	mockAgent := &MockUnsafeAgent{}
	
	// Init Session
	session := NewSession(mockDocker, mockAgent, tmpDir, "alpine")
	session.MaxIterations = 1
	
	// Capture output to verify blocking
	// Since we print to stdout, we can't easily capture it in a unit test without redirecting os.Stdout.
	// However, we can verify the behavior by checking if the DB has the observation.
	// If blocked, it shouldn't be saved to DB. (Our logic: Scan -> If Fail Continue -> Save DB)
	
	ctx := context.Background()
	if err := session.RunLoop(ctx); err != nil {
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
