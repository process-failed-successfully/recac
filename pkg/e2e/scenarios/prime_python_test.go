package scenarios

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrimePythonScenario_Verify_2(t *testing.T) {
	s := &PrimePythonScenario{}

	assert.Equal(t, "prime-python", s.Name())
	assert.NotEmpty(t, s.Description())
	assert.NotEmpty(t, s.AppSpec("url"))

	tickets := s.Generate("id", "url")
	assert.Len(t, tickets, 1)

	dir := setupGitRepo(t)

	// We need remote origin
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	_ = exec.Command("git", "-C", dir, "branch", "agent/PRIMES").Run()
	_ = exec.Command("git", "-C", dir, "push", "origin", "agent/PRIMES").Run()

	// 1. Test missing ticket keys
	err := s.Verify(dir, nil)
	assert.ErrorContains(t, err, "PRIMES ticket key not found")

	// 2. Test missing branch
	err = s.Verify(dir, map[string]string{"PRIMES": "UNKNOWN"})
	assert.ErrorContains(t, err, "specific branch for UNKNOWN not found")

	// Create a dummy json file that is invalid
	_ = os.WriteFile(filepath.Join(dir, "primes.json"), []byte("invalid"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "primes.py"), []byte("print('hello')"), 0644)

	// Will try to run python script, which doesn't produce the valid json
	err = s.Verify(dir, map[string]string{"PRIMES": "PRIMES"})
	assert.ErrorContains(t, err, "failed to parse JSON from primes.json")

	// Create a valid json file with incorrect primes count
	_ = os.WriteFile(filepath.Join(dir, "primes.json"), []byte(`{"primes": [2, 3, 5]}`), 0644)
	err = s.Verify(dir, map[string]string{"PRIMES": "PRIMES"})
	assert.ErrorContains(t, err, "expected 1229 primes, got 3")

	// Create a valid json file with correct primes
	primes := make([]int, 1229)
	primes[0] = 2
	primes[1228] = 9973
	data, _ := json.Marshal(map[string]interface{}{"primes": primes})
	_ = os.WriteFile(filepath.Join(dir, "primes.json"), data, 0644)

	err = s.Verify(dir, map[string]string{"PRIMES": "PRIMES"})
	assert.NoError(t, err)

	// Create a valid json file with wrong first prime
	primes[0] = 3
	data, _ = json.Marshal(map[string]interface{}{"primes": primes})
	_ = os.WriteFile(filepath.Join(dir, "primes.json"), data, 0644)

	err = s.Verify(dir, map[string]string{"PRIMES": "PRIMES"})
	assert.ErrorContains(t, err, "first prime is not 2, got 3")

	// Create a valid json file with wrong last prime
	primes[0] = 2
	primes[1228] = 10000
	data, _ = json.Marshal(map[string]interface{}{"primes": primes})
	_ = os.WriteFile(filepath.Join(dir, "primes.json"), data, 0644)

	err = s.Verify(dir, map[string]string{"PRIMES": "PRIMES"})
	assert.ErrorContains(t, err, "found prime >= 10000: 10000")
}
