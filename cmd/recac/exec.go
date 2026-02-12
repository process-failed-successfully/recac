package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

var kubectlFactory = func() (KubectlExecClient, error) {
	return &DefaultKubectlClient{}, nil
}

type KubectlExecClient interface {
	FindPod(ctx context.Context, sessionName string) (string, error)
	ExecInteractive(ctx context.Context, podName string, cmd []string) error
}

type DefaultKubectlClient struct{}

func (k *DefaultKubectlClient) FindPod(ctx context.Context, sessionName string) (string, error) {
	// Check multiple label strategies
	labels := []string{
		fmt.Sprintf("ticket=%s", sessionName),
		fmt.Sprintf("app.kubernetes.io/instance=%s", sessionName),
		fmt.Sprintf("app.kubernetes.io/name=%s", sessionName),
	}

	for _, l := range labels {
		// We use -o jsonpath to get just the name, and check if it's Running
		cmd := exec.CommandContext(ctx, "kubectl", "get", "pods", "-l", l, "--field-selector=status.phase=Running", "-o", "jsonpath={.items[0].metadata.name}")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			podName := strings.TrimSpace(string(output))
			return podName, nil
		}
	}
	return "", fmt.Errorf("pod not found")
}

func (k *DefaultKubectlClient) ExecInteractive(ctx context.Context, podName string, cmd []string) error {
	args := append([]string{"exec", "-it", podName, "--"}, cmd...)
	command := exec.CommandContext(ctx, "kubectl", args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

var execCmd = &cobra.Command{
	Use:   "exec [session-name] [command...]",
	Short: "Execute a command in a running session's container or pod",
	Long: `Execute a command in a running session's container.
Supports Docker containers and Kubernetes pods (via ticket/instance labels).
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
			fmt.Fprintf(cmd.ErrOrStderr(), "Error creating session manager: %v\n", err)
			return
		}

		session, err := sm.LoadSession(sessionName)

		// If session is found locally and has a valid Docker ID, try Docker first
		if err == nil && session.ContainerID != "" && session.ContainerID != "local" {
			dockerCli, err := dockerFactory(session.Name)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error creating Docker client: %v\n", err)
				return
			}

			// Best-effort close
			if closer, ok := dockerCli.(interface{ Close() error }); ok {
				defer closer.Close()
			}

			if err := dockerCli.ExecInteractive(context.Background(), session.ContainerID, command); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error executing command: %v\n", err)
			}
			return
		}

		// Fallback: If session is "local" (maybe K8s) OR not found locally, try Kubernetes
		k8sCli, kErr := kubectlFactory()
		if kErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error creating kubectl client: %v\n", kErr)
			return
		}

		podName, kErr := k8sCli.FindPod(context.Background(), sessionName)
		if kErr == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Found pod '%s' for session '%s'. Connecting...\n", podName, sessionName)
			if err := k8sCli.ExecInteractive(context.Background(), podName, command); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error executing kubectl exec: %v\n", err)
			}
			return
		}

		// Final error reporting
		if err == nil {
			// Session was found locally but no pod found
			fmt.Fprintf(cmd.ErrOrStderr(), "Session '%s' is running locally (PID %d) but no associated K8s pod found. Local process attachment is not supported via 'exec'.\n", sessionName, session.PID)
		} else {
			// Session not found locally and no pod found
			fmt.Fprintf(cmd.ErrOrStderr(), "Session '%s' not found locally or in Kubernetes.\n", sessionName)
		}
	},
}
