package scenarios

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHelperProcess is used to mock exec.Command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	// Simulate git checkout success
	if os.Getenv("GO_HELPER_PROCESS_CMD") == "git" {
		return
	}

	// Simulate server running (just wait a bit)
	if os.Getenv("GO_HELPER_PROCESS_CMD") == "go" || os.Getenv("GO_HELPER_PROCESS_CMD") == "python3" {
		select {} // Block forever (simulating a server)
	}
}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "GO_HELPER_PROCESS_CMD=" + command}
	return cmd
}

func TestDistributedLogScenario_Verify_Success(t *testing.T) {
	// Setup mocks
	origExecCommand := execCommand
	origExecLookPath := execLookPath
	origHttpGet := httpGet
	origHttpPost := httpPost
	defer func() {
		execCommand = origExecCommand
		execLookPath = origExecLookPath
		httpGet = origHttpGet
		httpPost = origHttpPost
	}()

	execCommand = fakeExecCommand
	execLookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	// Mock HTTP responses
	httpGet = func(url string) (*http.Response, error) {
		body := ""
		if url == "http://localhost:8080/consume?offset=0" {
			body = "first-entry"
		} else if url == "http://localhost:8080/consume?offset=1" {
			body = "second-entry"
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}, nil
	}

	httpPost = func(url, contentType string, body io.Reader) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"offset": 0}`)),
		}, nil
	}

	// Setup temp git repo to pass branch check
	dir := setupGitRepo(t)

	// Setup remote
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	// Create and push master first
	exec.Command("git", "-C", dir, "push", "origin", "master").Run()

	// Create the specific agent branch required and push it
	exec.Command("git", "-C", dir, "branch", "agent/LOG-123").Run()
	exec.Command("git", "-C", dir, "push", "origin", "agent/LOG-123").Run()

	s := &DistributedLogScenario{}
	ticketKeys := map[string]string{"LOG": "LOG-123"}

	err := s.Verify(dir, ticketKeys)
	assert.NoError(t, err)
}

func TestDistributedLogScenario_Verify_BranchNotFound(t *testing.T) {
	// Setup mocks
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()
	execCommand = fakeExecCommand

	// Setup temp git repo WITHOUT agent branch
	dir := setupGitRepo(t)

	s := &DistributedLogScenario{}
	ticketKeys := map[string]string{"LOG": "LOG-123"}

	err := s.Verify(dir, ticketKeys)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "branch for LOG-123 not found")
}

func TestDistributedLogScenario_Verify_ServerStartFail(t *testing.T) {
	// Setup mocks
	origExecCommand := execCommand
	origExecLookPath := execLookPath
	origHttpGet := httpGet
	defer func() {
		execCommand = origExecCommand
		execLookPath = origExecLookPath
		httpGet = origHttpGet
	}()

	execCommand = fakeExecCommand
	execLookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	// Mock HTTP Get to always fail (server never ready)
	httpGet = func(url string) (*http.Response, error) {
		return nil, fmt.Errorf("connection refused")
	}

	// Setup temp git repo
	dir := setupGitRepo(t)
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()
	exec.Command("git", "-C", dir, "push", "origin", "master").Run()

	exec.Command("git", "-C", dir, "branch", "agent/LOG-123").Run()
	exec.Command("git", "-C", dir, "push", "origin", "agent/LOG-123").Run()

	s := &DistributedLogScenario{}
	ticketKeys := map[string]string{"LOG": "LOG-123"}

	err := s.Verify(dir, ticketKeys)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server failed to start")
}

func TestDistributedLogScenario_Verify_ProduceFail(t *testing.T) {
	// Setup mocks
	origExecCommand := execCommand
	origExecLookPath := execLookPath
	origHttpGet := httpGet
	origHttpPost := httpPost
	defer func() {
		execCommand = origExecCommand
		execLookPath = origExecLookPath
		httpGet = origHttpGet
		httpPost = origHttpPost
	}()

	execCommand = fakeExecCommand
	execLookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	// Mock HTTP Get (Ready check passes)
	httpGet = func(url string) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("ok")),
		}, nil
	}

	// Mock HTTP Post (Produce fails)
	httpPost = func(url, contentType string, body io.Reader) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewBufferString("error")),
		}, nil
	}

	// Setup temp git repo
	dir := setupGitRepo(t)
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()
	exec.Command("git", "-C", dir, "push", "origin", "master").Run()

	exec.Command("git", "-C", dir, "branch", "agent/LOG-123").Run()
	exec.Command("git", "-C", dir, "push", "origin", "agent/LOG-123").Run()

	s := &DistributedLogScenario{}
	ticketKeys := map[string]string{"LOG": "LOG-123"}

	err := s.Verify(dir, ticketKeys)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "produce failed with status 500")
}

func TestDistributedLogScenario_Verify_PersistenceFail(t *testing.T) {
	// Setup mocks
	origExecCommand := execCommand
	origExecLookPath := execLookPath
	origHttpGet := httpGet
	origHttpPost := httpPost
	defer func() {
		execCommand = origExecCommand
		execLookPath = origExecLookPath
		httpGet = origHttpGet
		httpPost = origHttpPost
	}()

	execCommand = fakeExecCommand
	execLookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	// Mock HTTP responses
	httpGet = func(url string) (*http.Response, error) {
		// Initial checks OK
		if url == "http://localhost:8080/consume?offset=0" {
			// body = "first-entry"
		}
		// But after restart, data is lost (simulating failed persistence)
		// We can't easily distinguish "before restart" and "after restart" in this simple mock
		// without keeping state. Let's use a counter.
		// Actually, Verify calls consume(0) then consume(1) after restart.
		// Before restart it only calls consume for readiness check.
		// So if we make consume(0) return wrong data, it fails persistence check.

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("wrong-data")),
		}, nil
	}

	// Override readiness check to return success (ignoring body content for readiness usually, but Verify checks if err==nil)
	// Wait, Verify loop: `resp, err := http.Get(serverURL + "/consume?offset=0")` if err == nil -> ready.

	// We need stateful mock
	callCount := 0
	httpGet = func(url string) (*http.Response, error) {
		callCount++
		// Readiness checks (2 calls, one for each startServer)
		if callCount <= 2 {
			// Wait, startServer loops up to 20 times. Ideally returns quickly.
			// Let's assume it returns on first try.
			// Verify calls startServer -> loop consume(0).
			// Then produce.
			// Then kill.
			// Then startServer -> loop consume(0).
			// Then consume(0), consume(1).

			// Simple hack: if url contains consume, return success.
			// But for verification steps, we need specific return.
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("ok")),
			}, nil
		}

		// This is the persistence check calls
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("missing-data")),
		}, nil
	}

	httpPost = func(url, contentType string, body io.Reader) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"offset": 0}`)),
		}, nil
	}

	// Setup temp git repo
	dir := setupGitRepo(t)
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()
	exec.Command("git", "-C", dir, "push", "origin", "master").Run()

	exec.Command("git", "-C", dir, "branch", "agent/LOG-123").Run()
	exec.Command("git", "-C", dir, "push", "origin", "agent/LOG-123").Run()

	s := &DistributedLogScenario{}
	ticketKeys := map[string]string{"LOG": "LOG-123"}

	err := s.Verify(dir, ticketKeys)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "persistence check failed")
}
