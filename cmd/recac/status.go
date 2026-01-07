package main

import (
	"fmt"
	"io"
	"recac/internal/ui"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of RECAC sessions and environment",
	Long:  `Displays a summary of all running and completed RECAC sessions, checks the Docker environment, and shows key configuration values.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := showStatus(cmd.OutOrStdout()); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			exit(1)
		}
	},
}

func showStatus(out io.Writer) error {
	status, err := ui.GetStatus()
	if err != nil {
		return err
	}
	fmt.Fprint(out, status)
	return nil
}
