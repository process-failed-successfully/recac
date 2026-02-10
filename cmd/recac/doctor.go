package main

import (
	"context"
	"fmt"
	"recac/internal/doctor"
	"time"

	"github.com/spf13/cobra"
)

var doctorJson bool

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolVar(&doctorJson, "json", false, "Output results in JSON format")
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the RECAC environment for potential issues",
	Long:  `Runs a series of checks to ensure that the RECAC environment is set up correctly. This includes checking for system configuration, dependencies, Docker status, network connectivity, and API authentication.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		d := doctor.NewDoctor()
		d.AddCheck(doctor.NewSystemCheck())
		d.AddCheck(doctor.NewConfigCheck())
		d.AddCheck(doctor.NewDependencyCheck("git"))
		d.AddCheck(doctor.NewDependencyCheck("docker"))
		d.AddCheck(doctor.NewDockerCheck())
		d.AddCheck(doctor.NewNetworkCheck("https://www.google.com"))
		d.AddCheck(doctor.NewAuthCheck())

		results := d.RunChecks(ctx)

		if doctorJson {
			fmt.Fprintln(cmd.OutOrStdout(), doctor.FormatJSON(results))
		} else {
			fmt.Fprint(cmd.OutOrStdout(), doctor.FormatReport(results))
		}
	},
}
