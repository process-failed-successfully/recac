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
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Long:  `Print the version information for recac CLI`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("recac version %s\n", version)
			fmt.Printf("Commit: %s\n", commit)
			fmt.Printf("Build Date: %s\n", date)
			fmt.Printf("Go Version: %s\n", runtime.Version())
			fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}

	rootCmd.AddCommand(versionCmd)
}