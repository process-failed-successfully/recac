package scenarios

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockRedisServer(t *testing.T, delay time.Duration) net.Listener {
	l, err := net.Listen("tcp", "localhost:6379")
	require.NoError(t, err)

	go func() {
		defer l.Close()
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		time.Sleep(delay) // simulate boot time

		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}

			// very naive mock for the testRESP flow
			if line == "*1\r\n" {
				// PING
				reader.ReadString('\n') // $4
				reader.ReadString('\n') // PING
				conn.Write([]byte("+PONG\r\n"))
			} else if line == "*3\r\n" {
				// SET foo bar
				for i := 0; i < 6; i++ {
					reader.ReadString('\n')
				}
				conn.Write([]byte("+OK\r\n"))
			} else if line == "*2\r\n" {
				// GET
				reader.ReadString('\n')           // $3
				reader.ReadString('\n')           // GET
				reader.ReadString('\n')           // $length
				key, _ := reader.ReadString('\n') // key

				if key == "foo\r\n" {
					conn.Write([]byte("$3\r\nbar\r\n"))
				} else if key == "temp\r\n" {
					conn.Write([]byte("$-1\r\n"))
				}
			} else if line == "*5\r\n" {
				// SET temp val PX 100
				for i := 0; i < 10; i++ {
					reader.ReadString('\n')
				}
				conn.Write([]byte("+OK\r\n"))
			}
		}
	}()

	return l
}

func TestRedisChallengeScenario_Verify(t *testing.T) {
	s := &RedisChallengeScenario{}

	assert.Equal(t, "redis-challenge", s.Name())
	assert.NotEmpty(t, s.Description())
	assert.NotEmpty(t, s.AppSpec("url"))

	tickets := s.Generate("id", "url")
	assert.Len(t, tickets, 1)

	dir := setupGitRepo(t)

	// We need remote origin
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	_ = exec.Command("git", "-C", dir, "branch", "agent/REDIS").Run()
	_ = exec.Command("git", "-C", dir, "push", "origin", "agent/REDIS").Run()

	// 1. Test missing ticket keys
	err := s.Verify(dir, nil)
	assert.ErrorContains(t, err, "REDIS ticket key not found")

	// 2. Test missing branch
	err = s.Verify(dir, map[string]string{"REDIS": "UNKNOWN"})
	assert.ErrorContains(t, err, "branch for UNKNOWN not found")

	// Create a dummy python file so it tries to run `python3 main.py`
	// but python3 won't actually start a server on 6379, it will just exit.
	// So `ready` will be false.
	_ = os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('hello')"), 0644)

	err = s.Verify(dir, map[string]string{"REDIS": "REDIS"})
	assert.ErrorContains(t, err, "server failed to start on localhost:6379")
}

func TestRedisChallengeScenario_testRESP(t *testing.T) {
	s := &RedisChallengeScenario{}

	l := mockRedisServer(t, 0)
	defer l.Close()

	err := s.testRESP("localhost:6379")
	assert.NoError(t, err)
}

func TestRedisChallengeScenario_Verify_PortInUse(t *testing.T) {
	s := &RedisChallengeScenario{}
	dir := setupGitRepo(t)
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()
	exec.Command("git", "-C", dir, "push", "origin", "master").Run()

	exec.Command("git", "-C", dir, "branch", "agent/REDIS").Run()
	exec.Command("git", "-C", dir, "push", "origin", "agent/REDIS").Run()

	l, err := net.Listen("tcp", "localhost:6379")
	require.NoError(t, err)
	defer l.Close()

	err = s.Verify(dir, map[string]string{"REDIS": "REDIS"})
	assert.ErrorContains(t, err, "port 6379 already in use")
}

func TestRedisChallengeScenario_Verify_NoRunCommand(t *testing.T) {
	// Setup mocks
	origExecCommand := execCommand
	origExecLookPath := execLookPath
	defer func() {
		execCommand = origExecCommand
		execLookPath = origExecLookPath
	}()

	execCommand = fakeExecCommand
	execLookPath = func(file string) (string, error) {
		return "", fmt.Errorf("not found")
	}

	s := &RedisChallengeScenario{}
	dir := setupGitRepo(t)
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()
	exec.Command("git", "-C", dir, "push", "origin", "master").Run()

	exec.Command("git", "-C", dir, "branch", "agent/REDIS").Run()
	exec.Command("git", "-C", dir, "push", "origin", "agent/REDIS").Run()

	err := s.Verify(dir, map[string]string{"REDIS": "REDIS"})
	assert.ErrorContains(t, err, "could not determine how to run the server")
}

func TestRedisChallengeScenario_Verify_PortInUse(t *testing.T) {
	s := &RedisChallengeScenario{}
	dir := setupGitRepo(t)
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()
	exec.Command("git", "-C", dir, "push", "origin", "master").Run()

	exec.Command("git", "-C", dir, "branch", "agent/REDIS").Run()
	exec.Command("git", "-C", dir, "push", "origin", "agent/REDIS").Run()

	l, err := net.Listen("tcp", "localhost:6379")
	require.NoError(t, err)
	defer l.Close()

	err = s.Verify(dir, map[string]string{"REDIS": "REDIS"})
	assert.ErrorContains(t, err, "port 6379 already in use")
}
