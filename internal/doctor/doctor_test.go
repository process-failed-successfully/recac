package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// Helper to mock exec.Command
// Since we can't easily mock the returned *exec.Cmd struct, we usually replace the variable.
// But exec.Command returns a struct with unexported fields, so we can only control its behavior via
// a fake exec.
// The "TestHelperProcess" pattern is standard in Go.

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	// Append to existing environment instead of replacing
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

// TestHelperProcess is not a real test, it's a helper process invoked by fakeExecCommand.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
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
			fmt.Fprintln(os.Stdout, "go version go1.21.0 linux/amd64")
			os.Exit(0)
		}
	case "docker":
		if len(args) > 0 && args[0] == "info" {
			// Check env to simulate failure
			if os.Getenv("DOCKER_FAIL") == "1" {
				fmt.Fprintln(os.Stderr, "Cannot connect to the Docker daemon")
				os.Exit(1)
			}
			fmt.Fprintln(os.Stdout, "Client: Docker Engine - Community")
			os.Exit(0)
		}
	case "git": // dependency check
		os.Exit(0)
	case "make": // dependency check
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "Unknown command %q\n", cmd)
	os.Exit(2)
}

func TestSystemCheck(t *testing.T) {
	// Override execCommand
	oldExec := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExec }()

	check := NewSystemCheck()
	res := check.Run(context.Background())

	assert.True(t, res.Passed)
	assert.Contains(t, res.Message, "Go: go version go1.21.0")
}

func TestDockerCheck(t *testing.T) {
	oldExec := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = oldExec }()

	t.Run("Success", func(t *testing.T) {
		check := NewDockerCheck()
		res := check.Run(context.Background())
		assert.True(t, res.Passed)
		assert.Equal(t, "Daemon is running", res.Message)
	})

	t.Run("Failure", func(t *testing.T) {
		os.Setenv("DOCKER_FAIL", "1")
		defer os.Unsetenv("DOCKER_FAIL")

		check := NewDockerCheck()
		res := check.Run(context.Background())
		assert.False(t, res.Passed)
		assert.Contains(t, res.Message, "daemon not running")
	})
}

func TestConfigCheck(t *testing.T) {
	// Mock Viper
	viper.Reset()

	// Mock os.Stat
	oldStat := osStat
	defer func() { osStat = oldStat }()

	t.Run("Missing File", func(t *testing.T) {
		viper.SetConfigFile("") // simulates no config file used
		check := NewConfigCheck()
		res := check.Run(context.Background())
		assert.False(t, res.Passed)
		assert.Contains(t, res.Message, "no config file found")
	})

	t.Run("File Not Exist", func(t *testing.T) {
		viper.SetConfigFile("dummy.yaml")
		osStat = func(name string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		}
		check := NewConfigCheck()
		res := check.Run(context.Background())
		assert.False(t, res.Passed)
		assert.Contains(t, res.Message, "does not exist")
	})

	t.Run("Success", func(t *testing.T) {
		viper.SetConfigFile("config.yaml")
		viper.Set("provider", "gemini")
		osStat = func(name string) (os.FileInfo, error) {
			return nil, nil // success
		}
		check := NewConfigCheck()
		res := check.Run(context.Background())
		assert.True(t, res.Passed)
		assert.Contains(t, res.Message, "Provider: gemini")
	})
}

func TestAuthCheck(t *testing.T) {
	viper.Reset()

	t.Run("Mock Provider Skip", func(t *testing.T) {
		viper.Set("provider", "mock")
		check := NewAuthCheck()
		res := check.Run(context.Background())
		assert.True(t, res.Passed)
		assert.True(t, res.Skipped)
	})

	t.Run("Missing Key", func(t *testing.T) {
		viper.Set("provider", "gemini")
		viper.Set("api_key", "")
		os.Unsetenv("GEMINI_API_KEY")

		check := NewAuthCheck()
		res := check.Run(context.Background())
		assert.False(t, res.Passed)
		assert.Contains(t, res.Message, "API key not found")
	})

	t.Run("Success with Env", func(t *testing.T) {
		viper.Set("provider", "gemini")
		os.Setenv("GEMINI_API_KEY", "123456")
		defer os.Unsetenv("GEMINI_API_KEY")

		check := NewAuthCheck()
		res := check.Run(context.Background())
		assert.True(t, res.Passed)
	})
}
