package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"recac/internal/docker"
)

// checkDockerCmd represents the check-docker command
var checkDockerCmd = &cobra.Command{
	Use:   "check-docker",
	Short: "Check if Docker daemon is running",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := docker.NewClient()
		if err != nil {
			fmt.Printf("Error creating docker client: %v\n", err)
			os.Exit(1)
		}
		defer client.Close()

		if err := client.CheckDaemon(context.Background()); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Success: Docker daemon is running.")
	},
}

func init() {
	rootCmd.AddCommand(checkDockerCmd)
}

