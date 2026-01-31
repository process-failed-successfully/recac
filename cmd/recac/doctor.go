package main

import (
	"fmt"
	"recac/internal/ui"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var doctorFix bool

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Attempt to automatically fix issues")
	rootCmd.AddCommand(doctorCmd)
}

var diagnoseFunc = ui.Diagnose

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the RECAC environment for potential issues",
	Long:  `Runs a series of checks to ensure that the RECAC environment is set up correctly. This includes checking for a valid configuration file, required dependencies like git and docker, and connectivity to the Docker daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		diagnostics := diagnoseFunc()

		fmt.Fprintln(cmd.OutOrStdout(), "RECAC Doctor")
		fmt.Fprintln(cmd.OutOrStdout(), "------------")

		hasFailures := false
		for _, d := range diagnostics {
			symbol := "[?]"
			if d.Status == "PASS" {
				symbol = "[✔]"
			} else if d.Status == "FAIL" {
				symbol = "[✖]"
				hasFailures = true
			} else if d.Status == "WARN" {
				symbol = "[!]"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", symbol, d.Name, d.Message)
		}

		if !hasFailures {
			fmt.Fprintln(cmd.OutOrStdout(), "\nEverything looks good!")
			return
		}

		if doctorFix {
			fmt.Fprintln(cmd.OutOrStdout(), "\nAttempting to fix issues...")
			for _, d := range diagnostics {
				if d.Status == "FAIL" && d.CanAutoFix {
					fmt.Fprintf(cmd.OutOrStdout(), "\nFixing %s...\n", d.Name)
					if err := applyFix(d); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Failed to fix %s: %v\n", d.Name, err)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "Successfully fixed %s.\n", d.Name)
					}
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nDone. Run doctor again to verify.")
		} else {
			// Check if any failures are fixable
			fixableCount := 0
			for _, d := range diagnostics {
				if d.Status == "FAIL" && d.CanAutoFix {
					fixableCount++
				}
			}
			if fixableCount > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d issue(s) can be automatically fixed. Run 'recac doctor --fix' to apply fixes.\n", fixableCount)
			}
		}
	},
}

func applyFix(d ui.Diagnostic) error {
	switch d.FixID {
	case "fix_config":
		return RunSetupWizard()
	case "fix_git_identity":
		return fixGitIdentity()
	default:
		return fmt.Errorf("unknown fix ID: %s", d.FixID)
	}
}

func fixGitIdentity() error {
	var name string
	err := askOneFunc(&survey.Input{
		Message: "Enter Git User Name:",
	}, &name)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	var email string
	err = askOneFunc(&survey.Input{
		Message: "Enter Git User Email:",
	}, &email)
	if err != nil {
		return err
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	if err := execCommand("git", "config", "--global", "user.name", name).Run(); err != nil {
		return fmt.Errorf("failed to set user.name: %w", err)
	}
	if err := execCommand("git", "config", "--global", "user.email", email).Run(); err != nil {
		return fmt.Errorf("failed to set user.email: %w", err)
	}
	return nil
}
