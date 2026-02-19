package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelperProcess_Upgrade is the mock process that responds to commands
func TestHelperProcess_Upgrade(t *testing.T) {
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
		if len(args) > 0 && args[0] == "list" {
			// Mock go list -u -m -json all
			fmt.Print(`
{
	"Path": "github.com/pkg/errors",
	"Version": "v0.8.1",
	"Update": {
		"Version": "v0.9.1"
	}
}
{
	"Path": "github.com/stretchr/testify",
	"Version": "v1.7.0"
}
`)
		} else if len(args) > 0 && args[0] == "get" {
			// Mock go get
			// Check args
			if len(args) > 1 {
				// e.g. "go get pkg@v1.0.0"
				fmt.Printf("upgrading %s\n", args[1])
			}
		}
	case "npm":
		if len(args) > 0 && args[0] == "outdated" {
			// Mock npm outdated --json
			fmt.Print(`{
  "react": {
    "current": "16.8.0",
    "wanted": "16.8.6",
    "latest": "18.2.0",
    "location": "node_modules/react"
  }
}`)
		} else if len(args) > 0 && args[0] == "install" {
			fmt.Println("npm install success")
		}
	case "git":
		// Mock git diff
		fmt.Println("diff --git a/go.mod b/go.mod")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n", cmd)
		os.Exit(2)
	}
}

func fakeExecCommand_Upgrade(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess_Upgrade", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestCheckUpdatesGo(t *testing.T) {
	// Override execCommand
	originalExecCommand := execCommand
	execCommand = fakeExecCommand_Upgrade
	defer func() { execCommand = originalExecCommand }()

	updates, err := checkUpdatesGo()
	require.NoError(t, err)
	assert.Len(t, updates, 1)
	assert.Equal(t, "github.com/pkg/errors", updates[0].Name)
	assert.Equal(t, "v0.9.1", updates[0].Latest)
	assert.Equal(t, "go", updates[0].Type)
}

func TestCheckUpdatesNpm(t *testing.T) {
	// Override execCommand
	originalExecCommand := execCommand
	execCommand = fakeExecCommand_Upgrade
	defer func() { execCommand = originalExecCommand }()

	updates, err := checkUpdatesNpm()
	require.NoError(t, err)
	assert.Len(t, updates, 1)
	assert.Equal(t, "react", updates[0].Name)
	assert.Equal(t, "18.2.0", updates[0].Latest)
	assert.Equal(t, "npm", updates[0].Type)
}

func TestApplyUpdate(t *testing.T) {
	// Override execCommand
	originalExecCommand := execCommand
	execCommand = fakeExecCommand_Upgrade
	defer func() { execCommand = originalExecCommand }()

	t.Run("Go", func(t *testing.T) {
		c := UpgradeCandidate{
			Name:   "github.com/pkg/errors",
			Latest: "v0.9.1",
			Type:   "go",
		}
		err := applyUpdate(c)
		assert.NoError(t, err)
	})

	t.Run("NPM", func(t *testing.T) {
		c := UpgradeCandidate{
			Name:   "react",
			Latest: "18.2.0",
			Type:   "npm",
		}
		err := applyUpdate(c)
		assert.NoError(t, err)
	})
}

func TestDetectUpdates(t *testing.T) {
	// Override execCommand
	originalExecCommand := execCommand
	execCommand = fakeExecCommand_Upgrade
	defer func() { execCommand = originalExecCommand }()

	// Create dummy go.mod and package.json to trigger checks
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	os.WriteFile("go.mod", []byte("module test"), 0644)
	os.WriteFile("package.json", []byte("{}"), 0644)

	candidates, err := detectUpdates()
	require.NoError(t, err)
	// Expect 2 updates (1 from mock Go, 1 from mock NPM)
	assert.Len(t, candidates, 2)
}

func TestVerifyAndFix(t *testing.T) {
	originalExec := executeShellCommand
	defer func() { executeShellCommand = originalExec }()

	executeShellCommand = func(cmd string) (string, error) {
		if strings.Contains(cmd, "go test") {
			return "PASS", nil
		}
		return "", nil
	}

	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	os.WriteFile("go.mod", []byte("module test"), 0644)

	cmd := &cobra.Command{}
	// Run
	err := verifyAndFix(cmd)
	assert.NoError(t, err)
}
