package scenarios

import (
	"bufio"
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
				for i:=0; i<6; i++ { reader.ReadString('\n') }
				conn.Write([]byte("+OK\r\n"))
			} else if line == "*2\r\n" {
				// GET
				reader.ReadString('\n') // $3
				reader.ReadString('\n') // GET
				reader.ReadString('\n') // $length
				key, _ := reader.ReadString('\n') // key

				if key == "foo\r\n" {
					conn.Write([]byte("$3\r\nbar\r\n"))
				} else if key == "temp\r\n" {
					conn.Write([]byte("$-1\r\n"))
				}
			} else if line == "*5\r\n" {
				// SET temp val PX 100
				for i:=0; i<10; i++ { reader.ReadString('\n') }
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

func TestRedisChallengeScenario_Verify_Success(t *testing.T) {
	s := &RedisChallengeScenario{}
	dir := setupGitRepo(t)

	// We need remote origin
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	_ = exec.Command("git", "-C", dir, "branch", "agent/REDIS").Run()
	_ = exec.Command("git", "-C", dir, "push", "origin", "agent/REDIS").Run()

	mockPython := `
import socket
import sys

def handle_client(conn):
    try:
        while True:
            data = conn.recv(1024)
            if not data:
                break
            req = data.decode('utf-8')

            # Simple manual parsing of test flow
            if "*1\r\n$4\r\nPING\r\n" in req:
                conn.sendall(b"+PONG\r\n")
            elif "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n" in req:
                conn.sendall(b"+OK\r\n")
            elif "*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n" in req:
                conn.sendall(b"$3\r\nbar\r\n")
            elif "*5\r\n$3\r\nSET\r\n$4\r\ntemp\r\n$3\r\nval\r\n$2\r\nPX\r\n$3\r\n100\r\n" in req:
                conn.sendall(b"+OK\r\n")
            elif "*2\r\n$3\r\nGET\r\n$4\r\ntemp\r\n" in req:
                conn.sendall(b"$-1\r\n")
    except Exception as e:
        print(f"Error: {e}")
    finally:
        conn.close()

def main():
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    try:
        s.bind(('localhost', 6379))
        s.listen(1)
        print("Listening on 6379")
        while True:
            conn, addr = s.accept()
            handle_client(conn)
    except KeyboardInterrupt:
        sys.exit(0)
    finally:
        s.close()

if __name__ == '__main__':
    main()
`
	_ = os.WriteFile(filepath.Join(dir, "main.py"), []byte(mockPython), 0644)

	err := s.Verify(dir, map[string]string{"REDIS": "REDIS"})
	if err != nil {
		t.Logf("Verify Error: %v", err)
	}
	assert.NoError(t, err)
}

func TestRedisChallengeScenario_testRESP(t *testing.T) {
	s := &RedisChallengeScenario{}

	l := mockRedisServer(t, 0)
	defer l.Close()

	err := s.testRESP("localhost:6379")
	assert.NoError(t, err)
}
