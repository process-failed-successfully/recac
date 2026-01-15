package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Interact with the recac agent",
	Long:  `Provides a set of commands to interact with the recac agent.`,
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a recac agent session",
	Long:  `Executes the recac-agent binary with the specified parameters.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAgent(cmd, args)
	},
}

type Executor interface {
	Run() error
}

var commandExecutor = func(name string, arg ...string) Executor {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd
}

func runAgent(cmd *cobra.Command, _ []string) error {
	var commandArgs []string

	cmd.Flags().Visit(func(f *pflag.Flag) {
		value := f.Value.String()
		if f.Name == "verbose" {
			if value == "true" {
				commandArgs = append(commandArgs, "-v")
			}
			return
		}

		if value != "" && value != f.DefValue {
			commandArgs = append(commandArgs, fmt.Sprintf("--%s=%s", f.Name, value))
		}
	})

	fmt.Println("Running recac-agent with args:", strings.Join(commandArgs, " "))

	agentCmd := commandExecutor("recac-agent", commandArgs...)
	return agentCmd.Run()
}

func init() {
	// Add flags to runCmd
	runCmd.Flags().String("jira", "", "Jira Ticket ID to start session from (e.g. PROJ-123)")
	runCmd.Flags().String("repo-url", "", "Repository URL to clone")
	runCmd.Flags().String("summary", "", "Task summary")
	runCmd.Flags().BoolP("verbose", "v", false, "Enable verbose/debug logging")
	runCmd.Flags().Bool("stream", true, "Stream agent output to the console")
	runCmd.Flags().Bool("allow-dirty", false, "Allow running with uncommitted git changes")
	runCmd.Flags().String("image", "ghcr.io/process-failed-successfully/recac-agent:latest", "Docker image to use for the agent session")
	runCmd.Flags().String("provider", "", "Agent provider override")
	runCmd.Flags().String("model", "", "Agent model override")
	runCmd.Flags().Bool("mock", false, "Mock mode")
	runCmd.Flags().Int("max-iterations", 30, "Maximum number of iterations")

	agentCmd.AddCommand(runCmd)
	rootCmd.AddCommand(agentCmd)
}
