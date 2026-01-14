package main

import (
	"fmt"
	"os"
	"recac/internal/docker"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(execCmd)
}

var execCmd = &cobra.Command{
	Use:   "exec <session-id> [COMMAND]",
	Short: "Execute a command in a running session's container",
	Long:  `Execute a command in a running session's container. If no command is provided, it defaults to /bin/bash for an interactive shell.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sessionID := args[0]
		command := []string{"/bin/bash"}
		if len(args) > 1 {
			command = args[1:]
		}

		sm, err := sessionManagerFactory()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create session manager: %v\n", err)
			exit(1)
		}

		session, err := sm.LoadSession(sessionID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: session not found: %v\n", err)
			exit(1)
		}

		if session.ContainerID == "" {
			fmt.Fprintf(os.Stderr, "Error: session '%s' is not associated with a container\n", sessionID)
			exit(1)
		}

		dockerClient, err := docker.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create docker client: %v\n", err)
			exit(1)
		}

		err = dockerClient.ExecInContainer(cmd.Context(), session.ContainerID, command, os.Stdout, os.Stderr, os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to exec in container: %v\n", err)
			exit(1)
		}
	},
}
