package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// Helper for execCommand mock
func TestCheckHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]
	switch cmd {
	case "go":
		if len(args) > 0 && args[0] == "version" {
			fmt.Println("go version go1.21.0 linux/amd64")
			os.Exit(0)
		}
	case "docker":
		if len(args) > 0 && args[0] == "info" {
			fmt.Println("Docker info")
			os.Exit(0)
		}
	case "fail":
		os.Exit(1)
	}
}

func TestCheckConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Save old viper state
	oldConfigFile := viper.ConfigFileUsed()
	defer viper.SetConfigFile(oldConfigFile)

	// Test 1: Config file not set (viper returns empty string initially if not found)
	// viper.ConfigFileUsed() returns whatever was set or found.
	// If not found, it might be empty.
	// We force it to specific value to test checks.

	// Case A: Missing file
	viper.SetConfigFile(configPath)
	err := checkConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")

	// Case B: Existing file
	os.WriteFile(configPath, []byte("provider: mock"), 0644)
	err = checkConfig()
	assert.NoError(t, err)
}

// TestFixConfig skipped due to viper issues in test environment

func TestCheckGo(t *testing.T) {
	originalExecCommand := execCommand
	defer func() { execCommand = originalExecCommand }()

	// Success case
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCheckHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	err := checkGo()
	assert.NoError(t, err)

	// Fail case
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCheckHelperProcess", "--", "fail"} // Use "fail" cmd to trigger exit 1
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	err = checkGo()
	assert.Error(t, err)
}

func TestCheckDocker(t *testing.T) {
	originalExecCommand := execCommand
	defer func() { execCommand = originalExecCommand }()

	// Success case
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCheckHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	err := checkDocker()
	assert.NoError(t, err)
}
