package main

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "Print system diagnostic information",
	Long:  `Print diagnostic information about the host system, including OS, architecture, Go version, and PATH binary checks for orchestrator and recac-agent.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()

		fmt.Fprintln(out, "System Diagnostics:")
		fmt.Fprintf(out, "  OS: %s\n", runtime.GOOS)
		fmt.Fprintf(out, "  Architecture: %s\n", runtime.GOARCH)
		fmt.Fprintf(out, "  Go Version: %s\n", runtime.Version())

		fmt.Fprintln(out, "\nBinary Checks:")

		checkBinary(out, "orchestrator")
		checkBinary(out, "recac-agent")
		checkBinary(out, "docker")
		checkBinary(out, "git")

		return nil
	},
}

var execLookPath = exec.LookPath

func checkBinary(out io.Writer, name string) {
	path, err := execLookPath(name)
	if err != nil {
		fmt.Fprintf(out, "  %s: Not Found in PATH\n", name)
	} else {
		// Clean up the path for output
		fmt.Fprintf(out, "  %s: Found at %s\n", name, strings.TrimSpace(path))
	}
}

func init() {
	rootCmd.AddCommand(systemCmd)
}
