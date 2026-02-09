package docker

import (
	"bufio"
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

type NopConn struct{}

func (NopConn) Read(b []byte) (n int, err error) { return 0, io.EOF }
func (NopConn) Write(b []byte) (n int, err error) { return len(b), nil }
func (NopConn) Close() error                     { return nil }
func (NopConn) LocalAddr() net.Addr              { return nil }
func (NopConn) RemoteAddr() net.Addr             { return nil }
func (NopConn) SetDeadline(t time.Time) error    { return nil }
func (NopConn) SetReadDeadline(t time.Time) error { return nil }
func (NopConn) SetWriteDeadline(t time.Time) error { return nil }

func TestExecInteractive(t *testing.T) {
	client, mock := NewMockClient()

	var execConfig container.ExecOptions
	var execStartConfig container.ExecStartOptions

	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		execConfig = config
		return types.IDResponse{ID: "interactive-exec-id"}, nil
	}

	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		execStartConfig = config
		return types.HijackedResponse{
			Conn:   NopConn{},
			Reader: bufio.NewReader(strings.NewReader("")),
		}, nil
	}

	cmd := []string{"/bin/bash"}
	err := client.ExecInteractive(context.Background(), "test-container", cmd)
	if err != nil {
		t.Fatalf("ExecInteractive failed: %v", err)
	}

	// Verify ExecOptions
	if !execConfig.AttachStdin {
		t.Error("Expected AttachStdin to be true")
	}
	if !execConfig.AttachStdout {
		t.Error("Expected AttachStdout to be true")
	}
	if !execConfig.AttachStderr {
		t.Error("Expected AttachStderr to be true")
	}
	if !execConfig.Tty {
		t.Error("Expected Tty to be true")
	}

	// Verify ExecStartOptions
	if !execStartConfig.Tty {
		t.Error("Expected ExecStartOptions.Tty to be true")
	}
}
