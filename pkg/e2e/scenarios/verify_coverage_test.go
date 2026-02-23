package scenarios

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createTestRepo(t *testing.T) string {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init", tmpDir)
	if err := cmd.Run(); err != nil {
		t.Skipf("git init failed: %v", err)
	}

	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Run()
	}

	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test User")
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("init"), 0644)
	git("add", ".")
	git("commit", "-m", "init")

	return tmpDir
}

func TestDistributedLogScenario_Verify_BranchNotFound(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	repo := createTestRepo(t)
	s := &DistributedLogScenario{}

	// Test missing ticket key
	err := s.Verify(repo, map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LOG ticket key not found")

	// Test branch not found
	err = s.Verify(repo, map[string]string{"LOG": "LOG-999"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "branch for LOG-999 not found")
}

func TestLoadBalancerScenario_Verify_BranchNotFound(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	repo := createTestRepo(t)
	s := &LoadBalancerScenario{}

	err := s.Verify(repo, map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LB ticket key not found")

	err = s.Verify(repo, map[string]string{"LB": "LB-999"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "branch for LB-999 not found")
}

func TestHTTPProxyScenario_Verify_BranchNotFound(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	repo := createTestRepo(t)
	s := &HTTPProxyScenario{}

	// HTTPProxyScenario Verify checks for any agent branch first
	err := s.Verify(repo, map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no agent branches found")

	err = s.Verify(repo, map[string]string{"PROXY": "PROXY-999"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no agent branches found")
}

func TestRedisChallengeScenario_Verify_BranchNotFound(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	repo := createTestRepo(t)
	s := &RedisChallengeScenario{}

	// RedisChallengeScenario Verify seems to use ticket keys directly
	// Let's check implementation if needed, but assuming standard pattern:
	err := s.Verify(repo, map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "REDIS ticket key not found")

	err = s.Verify(repo, map[string]string{"REDIS": "REDIS-999"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "branch for REDIS-999 not found")
}

func TestSQLParserScenario_Verify_BranchNotFound(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	repo := createTestRepo(t)
	s := &SQLParserScenario{}

	// SQLParserScenario Verify falls back to generic agent branch if key missing
	err := s.Verify(repo, map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find any agent branch (fallback)")

	// If key provided but branch missing, it also falls back?
	// It tries getSpecificAgentBranch, if fails, logs warning and tries getAgentBranch
	// Key must match what Verify expects: "SQL-PARSER"
	err = s.Verify(repo, map[string]string{"SQL-PARSER": "SQL-999"})
	assert.Error(t, err)
	// It says "branch for %s not found and fallback failed"
	assert.Contains(t, err.Error(), "branch for SQL-999 not found and fallback failed")
}

func TestPrimePythonScenario_Verify_BranchNotFound(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	repo := createTestRepo(t)
	s := &PrimePythonScenario{}

	err := s.Verify(repo, map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PRIMES ticket key not found")

	err = s.Verify(repo, map[string]string{"PRIMES": "PRIMES-999"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "branch for PRIMES-999 not found")
}

func TestGenericScenario_Verify_BranchNotFound(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	repo := createTestRepo(t)

	config := GenericScenarioConfig{
		Name: "test",
		Tickets: []TicketTemplate{
			{ID: "TASK"},
		},
	}
	s := NewGenericScenario(config)

	// Verify uses keys from map if ticket ID is present
	// If map is empty, it skips branch checkout block for that key?
	// Let's see generic.go:
	// if key, ok := ticketKeys[firstID]; ok { ... }

	// So if key is missing, it skips checkout.
	// But then it runs validations. If validations are empty, it succeeds.

	// To test error, we must provide key but have branch missing.
	err := s.Verify(repo, map[string]string{"TASK": "TASK-999"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find agent branch")
}

func TestGenericScenario_Verify_Success(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	repo := createTestRepo(t)

	// Create a dummy file to verify
	os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello world"), 0644)

	config := GenericScenarioConfig{
		Name: "test-success",
		Validations: []ValidationStep{
			{
				Name: "Check File Exists",
				Type: ValidateFileExists,
				Path: "test.txt",
			},
			{
				Name: "Check Content",
				Type: ValidateFileContent,
				Path: "test.txt",
				ContentMustMatch: "hello",
			},
		},
	}
	s := NewGenericScenario(config)

	// Verify without tickets (skip branch checkout)
	err := s.Verify(repo, map[string]string{})
	assert.NoError(t, err)
}

func TestGenericScenario_Verify_Fail(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	repo := createTestRepo(t)

	config := GenericScenarioConfig{
		Name: "test-fail",
		Validations: []ValidationStep{
			{
				Name: "Check Missing File",
				Type: ValidateFileExists,
				Path: "missing.txt",
			},
		},
	}
	s := NewGenericScenario(config)

	err := s.Verify(repo, map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
}
