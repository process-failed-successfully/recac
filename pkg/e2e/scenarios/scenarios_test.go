package scenarios

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDistributedLogScenario_Generate(t *testing.T) {
	s := &DistributedLogScenario{}

	if s.Name() != "distributed-log" {
		t.Errorf("Expected name distributed-log")
	}

	if s.Description() == "" {
		t.Error("Expected description")
	}

	spec := s.AppSpec("http://repo")
	if spec == "" {
		t.Error("Expected spec")
	}

	tickets := s.Generate("uid", "http://repo")
	if len(tickets) != 1 {
		t.Errorf("Expected 1 ticket, got %d", len(tickets))
	}

	if tickets[0].ID != "LOG" {
		t.Errorf("Expected ticket LOG")
	}
}

func TestLoadBalancerScenario_Generate(t *testing.T) {
	s := &LoadBalancerScenario{}

	if s.Name() != "load-balancer" {
		t.Errorf("Expected name load-balancer")
	}

	tickets := s.Generate("uid", "http://repo")
	if len(tickets) != 1 {
		t.Errorf("Expected 1 ticket, got %d", len(tickets))
	}
}

func TestPrimePythonScenario_Generate(t *testing.T) {
	s := &PrimePythonScenario{}

	if s.Name() != "prime-python" {
		t.Errorf("Expected name prime-python")
	}

	tickets := s.Generate("uid", "http://repo")
	if len(tickets) != 1 {
		t.Errorf("Expected 1 ticket, got %d", len(tickets))
	}
}

func TestPrimePythonScenario_Verify_Mock(t *testing.T) {
	// Mocks the verification logic without needing real git/python env
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	s := &PrimePythonScenario{}
	tmpDir := t.TempDir()

	// Init minimal repo to satisfy git check
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create dummy file
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("init"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run()

	// Create branch
	branch := "agent/PRIMES-123"
	exec.Command("git", "-C", tmpDir, "checkout", "-b", branch).Run()

	// Create python script that works
	script := `
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

if __name__ == "__main__":
    import sys
    import json
    count = 0
    num = 2
    limit = 10000
    if len(sys.argv) > 1:
        limit = int(sys.argv[1])

    primes = []
    # If Verify calls without args, it expects limit=10000 by default (from implementation)
    # But Verify implementation passes "10000" usually?
    # Let's check PrimePythonScenario.Verify in code.
    # It calls python3 primes.py 10000.
    # But here I am mocking it.
    # The real Verify code expects 1229 primes for limit 10000.

    target_limit = limit

    curr = 2
    while curr <= target_limit:
        if is_prime(curr):
            primes.append(curr)
            print(curr)
        curr += 1

    # Write to primes.json as expected by Verify
    with open("primes.json", "w") as f:
        json.dump({"primes": primes}, f)
`
	os.WriteFile(filepath.Join(tmpDir, "primes.py"), []byte(script), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "impl").Run()

	// Setup remote (local)
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", tmpDir, "remote", "add", "origin", remoteDir).Run()
	exec.Command("git", "-C", tmpDir, "push", "origin", branch).Run()

	// Test Verify
	// It requires python3 installed. If not, skip.
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found")
	}

	ticketKeys := map[string]string{"PRIMES": "PRIMES-123"}
	// We need to inject a timeout context or ensure it returns fast.
	// The real Verify implementation uses context.WithTimeout.

	err := s.Verify(tmpDir, ticketKeys)
	if err != nil {
		t.Errorf("Verify failed: %v", err)
	}
}

func TestHTTPProxyScenario_Generate(t *testing.T) {
	s := &HTTPProxyScenario{}

	if s.Name() != "http-proxy" {
		t.Errorf("Expected name http-proxy")
	}

	tickets := s.Generate("uid", "http://repo")
	if len(tickets) == 0 {
		t.Errorf("Expected tickets")
	}
}

func TestRedisChallengeScenario_Generate(t *testing.T) {
	s := &RedisChallengeScenario{}

	if s.Name() != "redis-challenge" {
		t.Errorf("Expected name redis-challenge")
	}

	tickets := s.Generate("uid", "http://repo")
	if len(tickets) != 1 {
		t.Errorf("Expected 1 ticket, got %d", len(tickets))
	}
}

func TestSQLParserScenario_Generate(t *testing.T) {
	s := &SQLParserScenario{}

	if s.Name() != "sql-parser" {
		t.Errorf("Expected name sql-parser")
	}

	tickets := s.Generate("uid", "http://repo")
	if len(tickets) != 1 {
		t.Errorf("Expected 1 ticket, got %d", len(tickets))
	}
}
