package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	version = "v0.2.0"
	commit  = "HEAD"
	date    = "2025-12-28"
)

func init() {
	rootCmd.AddCommand(NewVersionCmd())
}

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Long:  `Print the version information for recac CLI`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "recac version %s\n", version)
			fmt.Fprintf(cmd.OutOrStdout(), "Commit: %s\n", commit)
			fmt.Fprintf(cmd.OutOrStdout(), "Build Date: %s\n", date)
			fmt.Fprintf(cmd.OutOrStdout(), "Go Version: %s\n", runtime.Version())
			fmt.Fprintf(cmd.OutOrStdout(), "Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}
}
