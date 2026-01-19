package docker

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// fakeConn is a mock for net.Conn
type fakeConn struct {
	net.Conn
}

func (f *fakeConn) Close() error { return nil }

func TestClient_Exec(t *testing.T) {
	client, mock := NewMockClient()

	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		// Prepare a buffer with Docker-style multiplexed output
		var buf bytes.Buffer
		msg := "hello\n"

		// Stdout header (type 1). The last 4 bytes are BigEndian size.
		header := [8]byte{1, 0, 0, 0, 0, 0, 0, byte(len(msg))}
		buf.Write(header[:])
		buf.Write([]byte(msg))

		return types.HijackedResponse{
			Conn:   &fakeConn{},
			Reader: bufio.NewReader(&buf),
		}, nil
	}

	output, err := client.Exec(context.Background(), "container-id", []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if strings.TrimSpace(output) != "hello" {
		t.Errorf("Expected output 'hello', got %q", output)
	}
}
