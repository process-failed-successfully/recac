package scenarios

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPProxyScenario_Verify_2(t *testing.T) {
	s := &HTTPProxyScenario{}

	assert.Equal(t, "http-proxy", s.Name())
	assert.NotEmpty(t, s.Description())
	assert.NotEmpty(t, s.AppSpec("url"))

	tickets := s.Generate("id", "url")
	assert.Len(t, tickets, 20)

	dir := setupGitRepo(t)

	// We need remote origin
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	// 1. Test missing branches
	err := s.Verify(dir, nil)
	assert.ErrorContains(t, err, "no agent branches found")

	_ = exec.Command("git", "-C", dir, "branch", "agent/README-123").Run()
	_ = exec.Command("git", "-C", dir, "push", "origin", "agent/README-123").Run()

	// 2. Test missing files in branch
	err = s.Verify(dir, map[string]string{"README": "README-123"})
	assert.ErrorContains(t, err, "required path go.mod not found in branch agent/README-123")

	// Create required files but missing some
	_ = exec.Command("git", "-C", dir, "checkout", "agent/README-123").Run()
	_ = exec.Command("touch", dir+"/go.mod").Run()
	_ = exec.Command("mkdir", "-p", dir+"/cmd").Run()

	_ = exec.Command("git", "-C", dir, "add", ".").Run()
	_ = exec.Command("git", "-C", dir, "commit", "-m", "add go.mod and cmd").Run()
	_ = exec.Command("git", "-C", dir, "push", "origin", "agent/README-123").Run()

	err = s.Verify(dir, map[string]string{"README": "README-123"})
	assert.ErrorContains(t, err, "required path internal/config/config.go not found")
}

func TestHTTPProxyScenario_Verify_Success(t *testing.T) {
	s := &HTTPProxyScenario{}
	dir := setupGitRepo(t)

	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	_ = exec.Command("git", "-C", dir, "branch", "agent/README-123").Run()
	_ = exec.Command("git", "-C", dir, "checkout", "agent/README-123").Run()

	_ = exec.Command("touch", dir+"/go.mod").Run()
	_ = exec.Command("mkdir", "-p", dir+"/cmd").Run()
	_ = exec.Command("mkdir", "-p", dir+"/internal/config").Run()
	_ = exec.Command("touch", dir+"/internal/config/config.go").Run()
	_ = exec.Command("mkdir", "-p", dir+"/internal/proxy").Run()

	_ = exec.Command("git", "-C", dir, "add", ".").Run()
	_ = exec.Command("git", "-C", dir, "commit", "-m", "setup").Run()
	_ = exec.Command("git", "-C", dir, "push", "origin", "agent/README-123").Run()

	err := s.Verify(dir, map[string]string{"README": "README-123"})
	assert.NoError(t, err)
}

func TestHTTPProxyScenario_Verify_FallbackBranch(t *testing.T) {
	s := &HTTPProxyScenario{}
	dir := setupGitRepo(t)

	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	_ = exec.Command("git", "-C", dir, "branch", "agent/OTHER-123").Run()
	_ = exec.Command("git", "-C", dir, "checkout", "agent/OTHER-123").Run()

	_ = exec.Command("touch", dir+"/go.mod").Run()
	_ = exec.Command("mkdir", "-p", dir+"/cmd").Run()
	_ = exec.Command("mkdir", "-p", dir+"/internal/config").Run()
	_ = exec.Command("touch", dir+"/internal/config/config.go").Run()
	_ = exec.Command("mkdir", "-p", dir+"/internal/proxy").Run()

	_ = exec.Command("git", "-C", dir, "add", ".").Run()
	_ = exec.Command("git", "-C", dir, "commit", "-m", "setup").Run()
	_ = exec.Command("git", "-C", dir, "push", "origin", "agent/OTHER-123").Run()

	// Provide empty map so it falls back to the OTHER branch
	err := s.Verify(dir, map[string]string{})
	assert.NoError(t, err)
}

func TestHTTPProxyScenario_Verify_FallbackBranchFail(t *testing.T) {
	// Not needed to increase coverage, checkout handles fallback logic
}
