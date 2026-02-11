package main

import (
	"context"
	"fmt"

	"recac/internal/docker"

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
	Use:     "exec [session-name] [command...]",
	Aliases: []string{"debug"},
	Short:   "Execute a command in a running session's container (Docker/K8s/Local)",
	Long: `Execute a command in a running session's environment.
Supports Docker containers, Kubernetes pods, and Local sessions.
If no command is provided, it will start an interactive shell (/bin/sh).`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]
		command := args[1:]
		if len(command) == 0 {
			command = []string{"/bin/sh"}
		}

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("error creating session manager: %w", err)
		}

		// 1. Try to load local session
		session, err := sm.LoadSession(sessionName)
		if err == nil {
			// Session found locally
			if session.ContainerID == "local" {
				// Local Execution
				cmd.Printf("Executing locally in workspace: %s\n", session.Workspace)
				c := execCommand(command[0], command[1:]...)
				c.Dir = session.Workspace
				c.Stdin = cmd.InOrStdin()
				c.Stdout = cmd.OutOrStdout()
				c.Stderr = cmd.ErrOrStderr()
				// TODO: We might want to inject session-specific env vars here if available
				return c.Run()
			} else if session.ContainerID != "" {
				// Docker Execution
				dockerCli, err := dockerFactory(session.Name)
				if err != nil {
					return fmt.Errorf("error creating Docker client: %w", err)
				}
				// We can't easily close the interface if it doesn't have Close(), but docker.Client does.
				// Since we use the interface, we cast or just rely on GC/OS cleanup for CLI.
				// But to be clean, let's check if closer, ok := dockerCli.(interface{ Close() error }); ok {
				// 	defer closer.Close()
				// }

				return dockerCli.ExecInteractive(context.Background(), session.ContainerID, command)
			}
			// If session found but no ContainerID, it might be a zombie or K8s-tracked session?
			// Fallthrough to K8s check.
		}

		// 2. Fallback to Kubernetes
		// If session not found or no ContainerID, try K8s
		k8sClient, err := k8sClientFactory()
		if err != nil {
			// If we can't create K8s client, and we already failed to load session, report original error
			return fmt.Errorf("session '%s' not found locally, and K8s client init failed: %w", sessionName, err)
		}

		// Search for pods with ticket=<sessionName>
		pods, err := k8sClient.ListPods(context.Background(), fmt.Sprintf("ticket=%s", sessionName))
		if err != nil {
			return fmt.Errorf("failed to list K8s pods: %w", err)
		}

		if len(pods) == 0 {
			// Try broader search? Or just fail.
			return fmt.Errorf("session '%s' not found locally or in Kubernetes", sessionName)
		}

		// Pick the first running pod
		podName := pods[0].Name
		namespace := pods[0].Namespace
		cmd.Printf("Found K8s pod: %s (Namespace: %s)\n", podName, namespace)

		// Construct kubectl command
		// kubectl exec -it <pod> -n <ns> -- <command>
		k8sArgs := []string{"exec", "-it", podName, "-n", namespace, "--"}
		k8sArgs = append(k8sArgs, command...)

		c := execCommand("kubectl", k8sArgs...)
		c.Stdin = cmd.InOrStdin()
		c.Stdout = cmd.OutOrStdout()
		c.Stderr = cmd.ErrOrStderr()
		return c.Run()
	},
}
