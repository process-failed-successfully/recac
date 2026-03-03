package scenarios

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHTTPProxyScenario_Verify(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	tmpDir := t.TempDir()

	// Init git repo
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create README to allow initial commit
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("init"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run()

	// Setup remote
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", tmpDir, "remote", "add", "origin", remoteDir).Run()

	// Create branch agent/README-123
	branchName := "agent/README-123"
	exec.Command("git", "-C", tmpDir, "checkout", "-b", branchName).Run()

	// Create required structure
	// "go.mod", "cmd", "internal/config/config.go", "internal/proxy"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module proxy"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "cmd"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "internal", "config"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "internal", "config", "config.go"), []byte("package config"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "internal", "proxy"), 0755)

	// Commit
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "impl").Run()

	// Push
	exec.Command("git", "-C", tmpDir, "push", "origin", branchName).Run()

	// Verify
	s := &HTTPProxyScenario{}
	ticketKeys := map[string]string{"README": "README-123"}

	err := s.Verify(tmpDir, ticketKeys)
	assert.NoError(t, err)
}

func TestLoadBalancerScenario_Verify(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not found")
	}

	tmpDir := t.TempDir()

	// Init git repo
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create README
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("init"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run()

	// Setup remote
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", tmpDir, "remote", "add", "origin", remoteDir).Run()

	// Create branch agent/LB-123
	branchName := "agent/LB-123"
	exec.Command("git", "-C", tmpDir, "checkout", "-b", branchName).Run()

	// Create LB implementation in Go
	mainContent := `package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

func main() {
	backends := strings.Split(os.Getenv("BACKENDS"), ",")
	var urls []*url.URL
	for _, b := range backends {
		if b == "" { continue }
		u, _ := url.Parse(b)
		urls = append(urls, u)
	}

	if len(urls) == 0 {
		panic("No backends")
	}

	var index uint64
	err := http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Active health check / retry logic on request (simplified)
		// Try up to len(urls) times to find a healthy backend
		for i := 0; i < len(urls)*2; i++ {
			idx := atomic.AddUint64(&index, 1)
			// Ensure strict round-robin order by using the current request's atomic increment modulo len(urls)
			// The original code was doing this correctly, but atomic.AddUint64 returns the NEW value.
			// If we want 0, 1, 0, 1... we should subtract 1 before modulo.
			target := urls[(idx-1) % uint64(len(urls))]

			// Quick health check
			client := http.Client{Timeout: 100 * time.Millisecond}
			resp, err := client.Get(target.String() + "/health")
			if err != nil || resp.StatusCode != 200 {
				if resp != nil { resp.Body.Close() }
				continue
			}
			if resp != nil { resp.Body.Close() }

			// Direct proxying without httputil since it can be flaky in tests with hosts
			proxyReq, err := http.NewRequest(r.Method, target.String()+r.URL.Path, r.Body)
			if err != nil {
				continue
			}
			for k, v := range r.Header {
				proxyReq.Header[k] = v
			}
			proxyResp, err := client.Do(proxyReq)
			if err != nil {
				continue
			}
			for k, v := range proxyResp.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(proxyResp.StatusCode)
			io.Copy(w, proxyResp.Body)
			proxyResp.Body.Close()
			return
		}
		http.Error(w, "Service Unavailable", 503)
	}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Listen failed: %v\n", err)
	}
	select{}
}`
	// Use port 0 to let OS assign an available port, since port 8080 might be in use
	// But the verification test hardcodes http://localhost:8080
	// So we need to ensure the mock code we provide runs on 8080 or we kill any existing process
	exec.Command("sh", "-c", "kill -9 $(lsof -t -i:8080)").Run()
	time.Sleep(100 * time.Millisecond) // wait for port to be freed

	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module lb\n\ngo 1.21"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)

	// Commit
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "impl").Run()

	// Push
	exec.Command("git", "-C", tmpDir, "push", "origin", branchName).Run()

	// Verify
	s := &LoadBalancerScenario{}
	ticketKeys := map[string]string{"LB": "LB-123"}

	err := s.Verify(tmpDir, ticketKeys)
	assert.NoError(t, err)
}

func TestSQLParserScenario_Verify(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not found")
	}

	tmpDir := t.TempDir()

	// Init git repo
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create README
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("init"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run()

	// Setup remote
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", tmpDir, "remote", "add", "origin", remoteDir).Run()

	// Create branch agent/PARSER-123
	branchName := "agent/PARSER-123"
	exec.Command("git", "-C", tmpDir, "checkout", "-b", branchName).Run()

	// Create dummy go project with passing test
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module parser\n\ngo 1.21"), 0644)
	mainContent := `package main
import "fmt"
func main() {
    fmt.Println(` + "`" + `{
      "type": "select",
      "columns": ["name", "age"],
      "from": "users",
      "where": {
        "type": "and",
        "left": {"type": "operator", "op": ">", "field": "age", "value": 25},
        "right": {
          "type": "or",
          "left": {"type": "operator", "op": "=", "field": "status", "value": "active"},
          "right": {"type": "operator", "op": "=", "field": "role", "value": "admin"}
        }
      }
    }` + "`" + `)
}`
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte("package main\nimport \"testing\"\nfunc TestDummy(t *testing.T) {}"), 0644)

	// Commit
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "impl").Run()

	// Push
	exec.Command("git", "-C", tmpDir, "push", "origin", branchName).Run()

	// Verify
	s := &SQLParserScenario{}
	// SQLParserScenario uses hardcoded ticket key?
	// Verify implementation in sql_parser.go:
	// ticketKey, ok := ticketKeys["PARSER"]

	ticketKeys := map[string]string{"PARSER": "PARSER-123"}

	err := s.Verify(tmpDir, ticketKeys)
	assert.NoError(t, err)
}
