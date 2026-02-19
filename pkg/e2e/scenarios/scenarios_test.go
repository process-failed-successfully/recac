package scenarios

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrimePythonScenario_Basics(t *testing.T) {
	s := &PrimePythonScenario{}
	assert.Equal(t, "prime-python", s.Name())
	assert.NotEmpty(t, s.Description())
	assert.NotEmpty(t, s.AppSpec("http://repo"))

	tickets := s.Generate("123", "http://repo")
	assert.Len(t, tickets, 1)
	assert.Equal(t, "PRIMES", tickets[0].ID)
}

func TestPrimePythonScenario_Verify(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found")
	}

	tmpDir := t.TempDir()

	// Init git repo
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create a dummy commit on main
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("init"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run()

	// Setup remote
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", tmpDir, "remote", "add", "origin", remoteDir).Run()

	// Checkout new branch
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "agent/PRIMES-123").Run()

	// Generate valid primes.json using python script
	script := `
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
`
	os.WriteFile(filepath.Join(tmpDir, "primes.py"), []byte(script), 0644)

	cmd := exec.Command("python3", "primes.py")
	cmd.Dir = tmpDir
	err := cmd.Run()
	assert.NoError(t, err)

	// Commit
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "add primes").Run()

	// Push to remote (so verify can find it)
	exec.Command("git", "-C", tmpDir, "push", "origin", "agent/PRIMES-123").Run()

	// Verify
	s := &PrimePythonScenario{}
	ticketKeys := map[string]string{"PRIMES": "PRIMES-123"}

	err = s.Verify(tmpDir, ticketKeys)
	assert.NoError(t, err)
}

func TestDistributedLogScenario_Basics(t *testing.T) {
	s := &DistributedLogScenario{}
	assert.Equal(t, "distributed-log", s.Name())
	assert.NotEmpty(t, s.Description())
	assert.NotEmpty(t, s.AppSpec("http://repo"))

	tickets := s.Generate("123", "http://repo")
	assert.NotEmpty(t, tickets)
}

func TestLoadBalancerScenario_Basics(t *testing.T) {
	s := &LoadBalancerScenario{}
	assert.Equal(t, "load-balancer", s.Name())
	assert.NotEmpty(t, s.Description())
	assert.NotEmpty(t, s.AppSpec("http://repo"))

	tickets := s.Generate("123", "http://repo")
	assert.NotEmpty(t, tickets)
}

func TestProxyScenario_Basics(t *testing.T) {
	s := &HTTPProxyScenario{}
	assert.Equal(t, "http-proxy", s.Name())
	assert.NotEmpty(t, s.Description())
	assert.NotEmpty(t, s.AppSpec("http://repo"))

	tickets := s.Generate("123", "http://repo")
	assert.NotEmpty(t, tickets)
}

func TestRedisChallengeScenario_Basics(t *testing.T) {
	s := &RedisChallengeScenario{}
	assert.Equal(t, "redis-challenge", s.Name())
	assert.NotEmpty(t, s.Description())
	assert.NotEmpty(t, s.AppSpec("http://repo"))

	tickets := s.Generate("123", "http://repo")
	assert.NotEmpty(t, tickets)
}

func TestSQLParserScenario_Basics(t *testing.T) {
	s := &SQLParserScenario{}
	assert.Equal(t, "sql-parser", s.Name())
	assert.NotEmpty(t, s.Description())
	assert.NotEmpty(t, s.AppSpec("http://repo"))

	tickets := s.Generate("123", "http://repo")
	assert.NotEmpty(t, tickets)
}
