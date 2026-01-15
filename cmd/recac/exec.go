package main

import (
	"context"
	"fmt"
	"os"

	"recac/internal/docker"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(execCmd)
}

var dockerFactory = func(project string) (DockerExecClient, error) {
	return docker.NewClient(project)
}

type DockerExecClient interface {
	ExecInteractive(ctx context.Context, containerID string, cmd []string) error
}


var execCmd = &cobra.Command{
	Use:   "exec [session-name] [command...]",
	Short: "Execute a command in a running session's container",
	Long: `Execute a command in a running session's container.
If no command is provided, it will start an interactive shell (/bin/sh).`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sessionName := args[0]
		command := args[1:]
		if len(command) == 0 {
			command = []string{"/bin/sh"}
		}

		sm, err := runner.NewSessionManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating session manager: %v\n", err)
			return
		}

		session, err := sm.LoadSession(sessionName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading session: %v\n", err)
			return
		}

		if session.ContainerID == "" {
			fmt.Fprintf(os.Stderr, "Session '%s' does not have a container ID. 'exec' is only supported for Docker-based sessions.\n", sessionName)
			return
		}

		dockerCli, err := docker.NewClient(session.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating Docker client: %v\n", err)
			return
		}
		defer dockerCli.Close()

		if err := dockerCli.ExecInteractive(context.Background(), session.ContainerID, command); err != nil {
			fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
		}
	},
}
