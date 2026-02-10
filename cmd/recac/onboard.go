package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"recac/internal/doctor"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var onboardExec = exec.Command

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Interactive onboarding wizard for new developers",
	Long:  `Runs a guided onboarding session to check the environment, explain the project, set up hooks, and suggest first tasks.`,
	RunE:  runOnboard,
}

func init() {
	rootCmd.AddCommand(onboardCmd)
}

func runOnboard(cmd *cobra.Command, args []string) error {
	// 1. Welcome
	fmt.Fprint(cmd.OutOrStdout(), `
  ____  _____ ____    _    ____
 |  _ \| ____/ ___|  / \  / ___|
 | |_) |  _|| |     / _ \| |
 |  _ <| |__| |___ / ___ \ |___
 |_| \_\_____\____/_/   \_\____|

Welcome to the team! 🚀
Let's get you set up and ready to code.

`)

	// 2. Doctor Check
	fmt.Fprintln(cmd.OutOrStdout(), "\n🏥 Checking Environment (Doctor)...")
	fmt.Fprintln(cmd.OutOrStdout(), "-----------------------------------")

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

	// 3. Project Info
	fmt.Fprintln(cmd.OutOrStdout(), "\n📂 Project Overview")
	fmt.Fprintln(cmd.OutOrStdout(), "-------------------")
	cwd, _ := os.Getwd()
	fmt.Fprintf(cmd.OutOrStdout(), "Location: %s\n", cwd)

	if remote, err := getGitRemote(); err == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Remote:   %s\n", remote)
	}
	if branch, err := getGitBranch(); err == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Branch:   %s\n", branch)
	}

	// 4. Hooks Setup
	fmt.Fprintln(cmd.OutOrStdout(), "")
	installHooksConfirm := false
	err := askOneFunc(&survey.Confirm{
		Message: "Install git pre-commit hooks (runs checks before commit)?",
		Default: true,
	}, &installHooksConfirm)
	if err != nil {
		return err // Handle interrupt
	}

	if installHooksConfirm {
		if err := installHooks(cwd, "default", true); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "❌ Failed to install hooks: %v\n", err)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Skipping hooks installation.")
	}

	// 5. First Task (Good First Issue)
	fmt.Fprintln(cmd.OutOrStdout(), "\n🎯 Finding a 'Good First Issue'...")
	fmt.Fprintln(cmd.OutOrStdout(), "----------------------------------")

	tasks, err := ScanForTodos(cwd)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to scan TODOs: %v\n", err)
	} else {
		var easyTasks []TodoItem

		// First pass: look for specific tags
		for _, t := range tasks {
			lower := strings.ToLower(t.Content)
			if strings.Contains(lower, "easy") || strings.Contains(lower, "good first issue") || strings.Contains(lower, "help wanted") {
				easyTasks = append(easyTasks, t)
			}
		}

		// Second pass: if none, just show first 3 TODOs
		if len(easyTasks) == 0 && len(tasks) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No tasks explicitly marked 'easy', but here are some open TODOs:")
			if len(tasks) > 3 {
				easyTasks = tasks[:3]
			} else {
				easyTasks = tasks
			}
		}

		if len(easyTasks) > 0 {
			for i, t := range easyTasks {
				if i >= 3 {
					break
				}
				fmt.Fprintf(cmd.OutOrStdout(), "- [%s:%d] %s\n", t.File, t.Line, t.Content)
			}
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "No TODOs found! Use 'recac tickets' to find work from Jira.")
		}
	}

	// 6. Next Steps
	fmt.Fprintln(cmd.OutOrStdout(), "\n🚀 Next Steps")
	fmt.Fprintln(cmd.OutOrStdout(), "-------------")
	fmt.Fprintln(cmd.OutOrStdout(), "1. Run 'recac quiz' to learn about the codebase.")
	fmt.Fprintln(cmd.OutOrStdout(), "2. Run 'recac doctor' to ensure the environment stays healthy.")
	fmt.Fprintln(cmd.OutOrStdout(), "3. Pick a task and start coding!")

	return nil
}

func getGitRemote() (string, error) {
	out, err := onboardExec("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func getGitBranch() (string, error) {
	out, err := onboardExec("git", "branch", "--show-current").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
