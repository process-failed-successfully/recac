package main

import (
	"fmt"
	"recac/internal/cmdutils"

	"github.com/spf13/cobra"
)

var doctorJSON bool

func init() {
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Output results in JSON format")
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose and verify the RECAC environment",
	Long: `Runs a comprehensive suite of checks to verify the RECAC environment, including:
- Configuration validity
- Dependencies (Git, Docker, Go)
- Connectivity (Docker Daemon, Jira, AI Provider)
- Workspace permissions`,
	Run: func(cmd *cobra.Command, args []string) {
		if doctorJSON {
			fmt.Fprintln(cmd.OutOrStdout(), cmdutils.GetDoctorJSON())
		} else {
			fmt.Fprint(cmd.OutOrStdout(), cmdutils.GetDoctor())
		}
	},
}
