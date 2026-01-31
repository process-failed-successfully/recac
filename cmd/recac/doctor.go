package main

import (
	"fmt"
	"recac/internal/ui"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var doctorFix bool

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Attempt to fix found issues")
	rootCmd.AddCommand(doctorCmd)

	// Inject the setup wizard into the UI package
	ui.ConfigFixer = func() error {
		return RunSetupWizard(true) // skipDoctor=true to prevent infinite loop
	}
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the RECAC environment for potential issues",
	Long:  `Runs a series of checks to ensure that the RECAC environment is set up correctly. This includes checking for a valid configuration file, required dependencies like git and docker, and connectivity to the Docker daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		runDoctor(cmd, doctorFix)
	},
}

func runDoctor(cmd *cobra.Command, fix bool) {
	fmt.Fprintln(cmd.OutOrStdout(), "RECAC Doctor")
	fmt.Fprintln(cmd.OutOrStdout(), "------------")

	results := ui.Diagnose(cmd.Context())

	for _, res := range results {
		symbol := "[✖]"
		if res.Status == "OK" {
			symbol = "[✔]"
		} else if res.Status == "WARN" {
			symbol = "[!]"
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", symbol, res.Component, res.Message)

		if fix && res.Status != "OK" && res.Fixable {
			confirm := false
			prompt := &survey.Confirm{
				Message: fmt.Sprintf("Attempt to fix %s?", res.Component),
				Default: true,
			}
			if err := survey.AskOne(prompt, &confirm); err == nil && confirm {
				if err := res.Fix(); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "    Failed to fix: %v\n", err)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "    [✔] Fixed!\n")
				}
			}
		}
	}
}
