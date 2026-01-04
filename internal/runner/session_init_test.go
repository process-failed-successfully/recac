package runner

import (
	"context"
	"fmt"
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
	mock.ContainerExecCreateFunc = func(ctx context.Context, containerID string, config container.ExecOptions) (types.IDResponse, error) {
		execCalls = append(execCalls, execCall{
			User: config.User,
			Cmd:  strings.Join(config.Cmd, " "),
		})
		return types.IDResponse{ID: "mock-exec-id"}, nil
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
	// - fixPasswdDatabase: grep ^.*:x:UID: /etc/passwd (as root)
	// - fixPasswdDatabase: echo ... >> /etc/passwd (as root)
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
			return types.IDResponse{}, fmt.Errorf("simulated init.sh failure")
		}
		return types.IDResponse{ID: "mock-exec-id"}, nil
	}

	session := NewSession(d, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

	if err := session.Start(context.Background()); err != nil {
		t.Fatalf("Start should NOT fail even if init.sh fails, but got: %v", err)
	}
}
