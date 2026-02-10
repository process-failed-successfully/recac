package main

import (
	"context"
	"fmt"
	"recac/internal/doctor"
	"time"

	"github.com/spf13/cobra"
)

var fixFlag bool

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "(Deprecated) Check dependencies and environment",
	Long: `(Deprecated) Perform pre-flight checks on the environment.
This command is deprecated in favor of 'recac doctor'.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️  'check' is deprecated. Please use 'recac doctor' instead.")
		fmt.Println("Running doctor checks...")

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
		fmt.Fprint(cmd.OutOrStdout(), doctor.FormatReport(results))

		// If fixFlag is set, we could try to implement fixes, but since we are deprecating,
		// we just warn that auto-fix is moved or removed.
		if fixFlag {
			fmt.Println("\n⚠️  --fix is no longer supported in 'check'. Please fix issues manually based on the report.")
		}
	},
}

func init() {
	checkCmd.Flags().BoolVar(&fixFlag, "fix", false, "Attempt to fix issues automatically (deprecated)")
	rootCmd.AddCommand(checkCmd)
}
