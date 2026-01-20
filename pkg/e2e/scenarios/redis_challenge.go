package scenarios

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

type RedisChallengeScenario struct{}

func (s *RedisChallengeScenario) Name() string {
	return "redis-challenge"
}

func (s *RedisChallengeScenario) Description() string {
	return "Build a TCP server that implements a subset of the Redis Protocol (RESP)."
}

func (s *RedisChallengeScenario) AppSpec(repoURL string) string {
	return fmt.Sprintf(`### ID:[REDIS] Redis Challenge

Build a TCP server that listens on port 6379 and implements the Redis Serialization Protocol (RESP).

The server MUST support the following commands:
1. PING: Returns +PONG\r\n
2. SET key value: Returns +OK\r\n and stores the value.
3. GET key: Returns the value as a bulk string ($<length>\r\n<value>\r\n) or $-1\r\n if not found.
4. SET key value PX <ms>: Implements expiry. The key should disappear after <ms> milliseconds.

Use any language (Python or Go preferred).
Ensure you use a bash block to create the source files.

Repo: %s`, repoURL)
}

func (s *RedisChallengeScenario) Generate(uniqueID string, repoURL string) []TicketSpec {
	return []TicketSpec{
		{
			ID:      "REDIS",
			Summary: fmt.Sprintf("[%s] Build a Redis-compatible TCP Server", uniqueID),
			Desc: fmt.Sprintf(`Build a TCP server that listens on port 6379 and implements the Redis Serialization Protocol (RESP).

The server MUST support the following commands:
1. PING: Returns +PONG\r\n
2. SET key value: Returns +OK\r\n and stores the value.
3. GET key: Returns the value as a bulk string ($<length>\r\n<value>\r\n) or $-1\r\n if not found.
4. SET key value PX <ms>: Implements expiry. The key should disappear after <ms> milliseconds.

Use any language (Python or Go preferred).
Ensure you use a bash block to create the source files.
Commit the code and ensure it is ready for execution.

Repo: %s`, repoURL),
			Type: "Task",
		},
	}
}

func (s *RedisChallengeScenario) Verify(repoPath string, ticketKeys map[string]string) error {
	ticketKey, ok := ticketKeys["REDIS"]
	if !ok {
		return fmt.Errorf("REDIS ticket key not found")
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

	// Try to start the server.
	// We'll try to find a main.go or main.py or similar.
	// Since we don't know the exact command, we'll try to let the agent define how to run it if possible,
	// but for E2E we usually assume standard entries.

	var runCmd *exec.Cmd
	if _, err := net.Dial("tcp", "localhost:6379"); err == nil {
		return fmt.Errorf("port 6379 already in use")
	}

	// Heuristic to find how to run
	if _, err := exec.LookPath("go"); err == nil {
		// Try go run .
		runCmd = exec.Command("go", "run", ".")
		runCmd.Dir = repoPath
	} else if _, err := exec.LookPath("python3"); err == nil {
		// Try python3 main.py
		runCmd = exec.Command("python3", "main.py")
		runCmd.Dir = repoPath
	}

	if runCmd == nil {
		return fmt.Errorf("could not determine how to run the server (no go or python3 found)")
	}

	// Use a separate process group or just kill it later
	if err := runCmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}
	defer func() {
		if runCmd.Process != nil {
			_ = runCmd.Process.Kill()
		}
	}()

	// Wait for server to be ready
	ready := false
	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", "localhost:6379", 1*time.Second)
		if err == nil {
			conn.Close()
			ready = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !ready {
		return fmt.Errorf("server failed to start on localhost:6379")
	}

	return s.testRESP("localhost:6379")
}

func (s *RedisChallengeScenario) testRESP(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// 1. PING
	fmt.Fprintf(conn, "*1\r\n$4\r\nPING\r\n")
	resp, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(resp, "PONG") {
		return fmt.Errorf("PING failed: expected +PONG, got %q (err: %v)", resp, err)
	}

	// 2. SET/GET
	fmt.Fprintf(conn, "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n")
	resp, _ = reader.ReadString('\n') // +OK

	fmt.Fprintf(conn, "*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n")
	_, _ = reader.ReadString('\n')      // $3
	line2, _ := reader.ReadString('\n') // bar
	if !strings.Contains(line2, "bar") {
		return fmt.Errorf("GET failed: expected bar, got %q", line2)
	}

	// 3. Expiry
	fmt.Fprintf(conn, "*5\r\n$3\r\nSET\r\n$4\r\ntemp\r\n$3\r\nval\r\n$2\r\nPX\r\n$3\r\n100\r\n")
	reader.ReadString('\n') // +OK

	time.Sleep(200 * time.Millisecond)
	fmt.Fprintf(conn, "*2\r\n$3\r\nGET\r\n$4\r\ntemp\r\n")
	resp, err = reader.ReadString('\n')
	if err != nil || !strings.Contains(resp, "$-1") {
		return fmt.Errorf("Expiry failed: expected $-1 for expired key, got %q", resp)
	}

	return nil
}

func init() {
	Register(&RedisChallengeScenario{})
}
