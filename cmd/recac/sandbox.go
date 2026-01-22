package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"recac/internal/docker"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	sandboxCmd.Flags().String("image", "", "Docker image to use (default: config.image or recac-agent:latest)")
	sandboxCmd.Flags().String("user", "root", "User to run as inside the container")
	rootCmd.AddCommand(sandboxCmd)
}

// SandboxDockerClient defines the interface for Docker operations needed by sandbox.
type SandboxDockerClient interface {
	RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, ports []string, user string) (string, error)
	ExecInteractive(ctx context.Context, containerID string, cmd []string) error
	StopContainer(ctx context.Context, containerID string) error
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	Close() error
}

var sandboxDockerFactory = func(project string) (SandboxDockerClient, error) {
	return docker.NewClient(project)
}

var sandboxCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "Start a disposable development environment",
	Long: `Starts a fresh Docker container with the current project mounted and drops you into a shell.
The container is ephemeral and will be removed when you exit the shell.

⚠️  WARNING: The current directory is mounted to /workspace inside the container.
Changes to files within /workspace WILL be reflected on your host machine.
This allows you to edit code locally and run it in the container, but be careful with file deletions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle signals for graceful cleanup
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			cancel() // Cancel context to trigger cleanup
		}()

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		projectName := filepath.Base(cwd)

		imageRef, _ := cmd.Flags().GetString("image")
		if imageRef == "" {
			imageRef = viper.GetString("config.image")
		}
		if imageRef == "" {
			imageRef = "ghcr.io/process-failed-successfully/recac-agent:latest"
		}

		user, _ := cmd.Flags().GetString("user")

		cli, err := sandboxDockerFactory(projectName)
		if err != nil {
			return fmt.Errorf("failed to create docker client: %w", err)
		}
		defer cli.Close()

		fmt.Fprintf(cmd.OutOrStdout(), "🚀 Starting sandbox environment...\n")
		fmt.Fprintf(cmd.OutOrStdout(), "   Image: %s\n", imageRef)
		fmt.Fprintf(cmd.OutOrStdout(), "   Mount: %s -> /workspace (RW)\n", cwd)
		fmt.Fprintf(cmd.OutOrStdout(), "   ⚠️  Changes to files in /workspace will persist on host!\n")

		// Start Container
		containerID, err := cli.RunContainer(ctx, imageRef, cwd, nil, nil, user)
		if err != nil {
			return fmt.Errorf("failed to start sandbox container: %w", err)
		}

		// Ensure cleanup
		defer func() {
			fmt.Fprintf(cmd.OutOrStdout(), "\n🧹 Cleaning up sandbox container %s...\n", containerID[:12])
			// Use a fresh context for cleanup in case original is cancelled
			cleanupCtx := context.Background()
			if err := cli.StopContainer(cleanupCtx, containerID); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to stop container: %v\n", err)
			}
			if err := cli.RemoveContainer(cleanupCtx, containerID, true); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to remove container: %v\n", err)
			}
		}()

		fmt.Fprintf(cmd.OutOrStdout(), "✅ Sandbox ready! Dropping into shell...\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Type 'exit' to quit.\n\n")

		// Interactive Shell
		shellCmd := []string{"/bin/bash"}
		// Fallback to sh if bash fails? RunContainer usually keeps it alive with sh.
		// Let's try bash, if it fails, ExecInteractive returns error.
		// Actually, we can just try to exec. If the image is alpine based, it might only have sh.
		// Most dev images have bash.

		err = cli.ExecInteractive(ctx, containerID, shellCmd)
		if err != nil {
			// If bash failed, try sh
			fmt.Fprintf(cmd.OutOrStdout(), "Bash not found or failed, falling back to sh...\n")
			err = cli.ExecInteractive(ctx, containerID, []string{"/bin/sh"})
		}

		if err != nil {
			return fmt.Errorf("session ended with error: %w", err)
		}

		return nil
	},
}
