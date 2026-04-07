package utils

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestOpenBrowser(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	var executedCommand string
	execCommand = func(name string, arg ...string) *exec.Cmd {
		executedCommand = name
		return exec.Command("echo") // using a harmless command
	}

	err := OpenBrowser("http://localhost:8080")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	switch runtime.GOOS {
	case "linux":
		if executedCommand != "xdg-open" {
			t.Errorf("Expected xdg-open, got %s", executedCommand)
		}
	case "windows":
		if executedCommand != "rundll32" {
			t.Errorf("Expected rundll32, got %s", executedCommand)
		}
	case "darwin":
		if executedCommand != "open" {
			t.Errorf("Expected open, got %s", executedCommand)
		}
	}
}

func TestOpenBrowser_AllPlatforms(t *testing.T) {
    origExecCommand := execCommand
    defer func() { execCommand = origExecCommand }()

    var executedCommand string
    execCommand = func(name string, arg ...string) *exec.Cmd {
        executedCommand = name
        return exec.Command("echo")
    }

    origOS := runtimeGOOS
    defer func() { runtimeGOOS = origOS }()

    platforms := []struct{
        os string
        expected string
    }{
        {"linux", "xdg-open"},
        {"windows", "rundll32"},
        {"darwin", "open"},
        {"unknown", ""},
    }

    for _, p := range platforms {
        t.Run(p.os, func(t *testing.T) {
            runtimeGOOS = p.os
            executedCommand = ""
            err := OpenBrowser("http://localhost:8080")
            if p.os == "unknown" {
                if err == nil {
                    t.Errorf("Expected error for unknown platform")
                }
            } else {
                if err != nil {
                    t.Fatalf("Expected no error, got %v", err)
                }
                if executedCommand != p.expected {
                    t.Errorf("Expected %s, got %s", p.expected, executedCommand)
                }
            }
        })
    }
}

func TestOpenBrowser_Error(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("this-command-does-not-exist-12345")
	}

	err := OpenBrowser("http://localhost:8080")
	if err == nil {
		t.Fatalf("Expected an error, got nil")
	}
}
