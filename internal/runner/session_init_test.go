package runner

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"recac/internal/docker"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type execCall struct {
	User string
	Cmd  string
}

func TestSession_Start_RunsInitScript(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create app_spec.txt (required by Start)
	specPath := filepath.Join(tmpDir, "app_spec.txt")
	os.WriteFile(specPath, []byte("test spec"), 0644)

	// 2. Create init.sh in workspace
	initPath := filepath.Join(tmpDir, "init.sh")
	os.WriteFile(initPath, []byte("#!/bin/sh\necho 'initializing'"), 0644)

	// 3. Setup Mock Docker
	d, mock := docker.NewMockClient()

	execCalls := []execCall{}
	// Map ID -> Command to determine exit code in Inspect
	execCmds := make(map[string]string)

	mock.ContainerExecCreateFunc = func(ctx context.Context, containerID string, config container.ExecOptions) (types.IDResponse, error) {
		cmdStr := strings.Join(config.Cmd, " ")
		execCalls = append(execCalls, execCall{
			User: config.User,
			Cmd:  cmdStr,
		})
		// Generate unique ID based on command to allow inspecting result
		id := fmt.Sprintf("exec-%d", len(execCalls))
		execCmds[id] = cmdStr
		return types.IDResponse{ID: id}, nil
	}

	// Mock Inspect to return failure for 'getent' (simulating missing user)
	mock.ContainerExecInspectFunc = func(ctx context.Context, execID string) (container.ExecInspect, error) {
		cmd, ok := execCmds[execID]
		exitCode := 0
		if ok && strings.Contains(cmd, "getent") {
			exitCode = 1 // User/Group missing
		}
		return container.ExecInspect{ExitCode: exitCode}, nil
	}

	// Mock Attach to return empty output for getent (to trigger user creation logic)
	// and default output for others
	mock.ContainerExecAttachFunc = func(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
		server, client := net.Pipe()
		cmd, _ := execCmds[execID]

		output := "Success: Mock command executed\n"
		if strings.Contains(cmd, "getent") {
			output = "" // Empty output for getent implies missing user/group
		}

		go func() {
			if output != "" {
				msg := []byte(output)
				header := make([]byte, 8)
				header[0] = 1 // stdout
				binary.BigEndian.PutUint32(header[4:], uint32(len(msg)))
				server.Write(header)
				server.Write(msg)
			}
			server.Close()
		}()
		return types.HijackedResponse{
			Conn:   client,
			Reader: bufio.NewReader(client),
		}, nil
	}

	// Mock ContainerCreate to return a valid ID
	mock.ContainerCreateFunc = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
		return container.CreateResponse{ID: "test-container"}, nil
	}

	// 4. Create and Start Session
	session := NewSession(d, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 5. Verify Exec Calls
	// Expected calls:
	// - fixPasswdDatabase: getent group 1001 (returns 1, empty output)
	// - fixPasswdDatabase: groupadd (triggered by failure)
	// - fixPasswdDatabase: getent passwd 1001 (returns 1, empty output)
	// - fixPasswdDatabase: useradd (triggered by failure)
	// - bootstrapGit: git config ... (3 calls, as root)
	// - runInitScript: chmod +x init.sh (as default user)
	// - runInitScript: ./init.sh (as default user)

	foundPasswdFix := false
	foundGitRoot := false
	foundChmod := false

	for _, call := range execCalls {
		if strings.Contains(call.Cmd, "useradd") && call.User == "root" {
			foundPasswdFix = true
		}
		if strings.Contains(call.Cmd, "git config") && call.User == "root" {
			foundGitRoot = true
		}
		if strings.Contains(call.Cmd, "chmod +x init.sh") {
			foundChmod = true
		}
		// init.sh runs async, so we can't reliably check for it here without a race condition.
		// if strings.Contains(call.Cmd, "./init.sh") { foundExec = true }
	}

	if !foundPasswdFix {
		t.Errorf("Expected useradd fix as root, but not found in %v", execCalls)
	}
	if !foundGitRoot {
		t.Errorf("Expected git config as root, but not found in %v", execCalls)
	}
	if !foundChmod {
		t.Errorf("Expected chmod +x init.sh call, but not found in %v", execCalls)
	}
	// if !foundExec { ... }
}

func TestSession_Start_NoInitScript(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "app_spec.txt")
	os.WriteFile(specPath, []byte("test spec"), 0644)

	d, mock := docker.NewMockClient()

	execCalls := []execCall{}
	mock.ContainerExecCreateFunc = func(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
		execCalls = append(execCalls, execCall{
			User: config.User,
			Cmd:  strings.Join(config.Cmd, " "),
		})
		return types.IDResponse{ID: "mock-exec-id"}, nil
	}

	session := NewSession(d, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify no init.sh calls
	for _, call := range execCalls {
		if strings.Contains(call.Cmd, "init.sh") {
			t.Errorf("Unexpected init.sh call: %s", call.Cmd)
		}
	}
}

func TestSession_Start_InitScriptFails(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "app_spec.txt")
	os.WriteFile(specPath, []byte("test spec"), 0644)

	initPath := filepath.Join(tmpDir, "init.sh")
	os.WriteFile(initPath, []byte("#!/bin/sh\nexit 1"), 0644)

	d, mock := docker.NewMockClient()

	mock.ContainerExecCreateFunc = func(ctx context.Context, containerID string, config container.ExecOptions) (types.IDResponse, error) {
		if strings.Contains(strings.Join(config.Cmd, " "), "./init.sh") {
			// Expected to be called
		}
		return types.IDResponse{ID: "mock-exec-id"}, nil
	}

	session := NewSession(d, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Start should NOT fail even if init.sh fails, but got: %v", err)
	}
}
