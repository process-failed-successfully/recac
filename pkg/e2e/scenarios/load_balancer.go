package scenarios

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type LoadBalancerScenario struct{}

func (s *LoadBalancerScenario) Name() string {
	return "load-balancer"
}

func (s *LoadBalancerScenario) Description() string {
	return "Build a Round-Robin HTTP load balancer with active health checks."
}

func (s *LoadBalancerScenario) AppSpec(repoURL string) string {
	return fmt.Sprintf(`### ID:[LB] Round-Robin Load Balancer

Build an HTTP load balancer that listens on port 8080 and distributes traffic across a list of backend servers using a Round-Robin algorithm.

Requirements:
1. Configuration: Accept a list of backend URLs (e.g., via environment variable BACKENDS="http://127.0.0.1:8081,http://127.0.0.1:8082").
2. Round-Robin: Distribute incoming requests evenly across the backends.
3. Health Checks: Actively check the health of each backend (e.g., GET /health). If a backend is down, stop sending traffic to it.
4. Recovery: If a backend comes back up, resume sending traffic to it.

Use any language (Python or Go preferred).
Ensure you use a bash block to create the source files.

Repo: %s`, repoURL)
}

func (s *LoadBalancerScenario) Generate(uniqueID string, repoURL string) []TicketSpec {
	return []TicketSpec{
		{
			ID:      "LB",
			Summary: fmt.Sprintf("[%s] Build a Round-Robin Load Balancer", uniqueID),
			Desc: fmt.Sprintf(`Build an HTTP load balancer that listens on port 8080 and distributes traffic across a list of backend servers using a Round-Robin algorithm.

Requirements:
1. Configuration: Accept a list of backend URLs (e.g., via environment variable BACKENDS="http://127.0.0.1:8081,http://127.0.0.1:8082").
2. Round-Robin: Distribute incoming requests evenly across the backends.
3. Health Checks: Actively check the health of each backend (e.g., GET /health). If a backend is down, stop sending traffic to it.
4. Recovery: If a backend comes back up, resume sending traffic to it.

Use any language (Python or Go preferred).
Ensure you use a bash block to create the source files.
Commit the code and ensure it is ready for execution.

Repo: %s`, repoURL),
			Type: "Task",
		},
	}
}

func (s *LoadBalancerScenario) Verify(repoPath string, ticketKeys map[string]string) error {
	ticketKey, ok := ticketKeys["LB"]
	if !ok {
		return fmt.Errorf("LB ticket key not found")
	}

	branch, err := getSpecificAgentBranch(repoPath, ticketKey)
	if err != nil {
		return fmt.Errorf("branch for %s not found: %w", ticketKey, err)
	}

	// Checkout branch
	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutCmd.Dir = repoPath
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to checkout %s: %v\nOutput: %s", branch, err, out)
	}

	// 1. Setup Mock Backends
	var mu sync.Mutex
	hits := make(map[string]int)

	createBackend := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				return
			}
			mu.Lock()
			hits[name]++
			mu.Unlock()
			fmt.Fprintf(w, "Hello from %s", name)
		}))
	}

	be1 := createBackend("be1")
	defer be1.Close()
	be2 := createBackend("be2")
	defer be2.Close()

	backends := fmt.Sprintf("%s,%s", be1.URL, be2.URL)

	// 2. Start the Load Balancer
	var runCmd *exec.Cmd
	// Heuristic to find how to run
	if _, err := exec.LookPath("go"); err == nil {
		runCmd = exec.Command("go", "run", ".")
		runCmd.Dir = repoPath
	} else if _, err := exec.LookPath("python3"); err == nil {
		runCmd = exec.Command("python3", "main.py")
		runCmd.Dir = repoPath
	}

	if runCmd == nil {
		return fmt.Errorf("could not determine how to run the LB")
	}

	runCmd.Env = append(runCmd.Environ(), "BACKENDS="+backends)
	if err := runCmd.Start(); err != nil {
		return fmt.Errorf("failed to start LB: %v", err)
	}
	defer func() {
		if runCmd.Process != nil {
			_ = runCmd.Process.Kill()
		}
	}()

	// Wait for LB to be ready
	lbURL := "http://localhost:8080"
	ready := false
	for i := 0; i < 20; i++ {
		resp, err := http.Get(lbURL)
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !ready {
		return fmt.Errorf("load balancer failed to start on %s", lbURL)
	}

	// 3. Test Round-Robin
	for i := 0; i < 10; i++ {
		resp, err := http.Get(lbURL)
		if err != nil {
			return fmt.Errorf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
	}

	mu.Lock()
	be1Hits := hits["be1"]
	be2Hits := hits["be2"]
	mu.Unlock()

	if be1Hits == 0 || be2Hits == 0 {
		return fmt.Errorf("Round-Robin failed: be1 hits=%d, be2 hits=%d", be1Hits, be2Hits)
	}
	fmt.Printf("Round-Robin check passed: be1=%d, be2=%d\n", be1Hits, be2Hits)

	// 4. Test Health Check / Failover
	be1.Close() // Kill BE1
	fmt.Println("Killed be1, waiting for health check to trigger cleanup...")

	// Give LB time to detect failure
	success := false
	for i := 0; i < 20; i++ {
		resp, err := http.Get(lbURL)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if strings.Contains(string(body), "be2") {
			success = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !success {
		return fmt.Errorf("failover failed: LB did not switch exclusively to be2")
	}

	return nil
}

func init() {
	Register(&LoadBalancerScenario{})
}
