package main

import (
	e2ecmd "recac/pkg/e2e/cmd"

	"github.com/spf13/cobra"
)

var (
	runSetupFunc   = e2ecmd.RunSetup
	runDeployFunc  = e2ecmd.RunDeploy
	runWaitFunc    = e2ecmd.RunWait
	runVerifyFunc  = e2ecmd.RunVerify
	runCleanupFunc = e2ecmd.RunCleanup
)

func init() {
	rootCmd.AddCommand(NewE2ECmd())
}

func NewE2ECmd() *cobra.Command {
	e2eCmd := &cobra.Command{
		Use:   "e2e",
		Short: "End-to-End testing utilities",
		Long:  `Utilities for running, setting up, and verifying End-to-End (E2E) test scenarios.`,
	}

	e2eCmd.AddCommand(&cobra.Command{
		Use:                "setup [flags]",
		Short:              "Setup an E2E scenario (Jira tickets, Git repo)",
		DisableFlagParsing: true,
		RunE: func(c *cobra.Command, args []string) error {
			return runSetupFunc(args)
		},
	})

	e2eCmd.AddCommand(&cobra.Command{
		Use:                "deploy [flags]",
		Short:              "Deploy the orchestrator and agents (Helm/Docker)",
		DisableFlagParsing: true,
		RunE: func(c *cobra.Command, args []string) error {
			return runDeployFunc(args)
		},
	})

	e2eCmd.AddCommand(&cobra.Command{
		Use:                "wait [flags]",
		Short:              "Wait for agent completion",
		DisableFlagParsing: true,
		RunE: func(c *cobra.Command, args []string) error {
			return runWaitFunc(args)
		},
	})

	e2eCmd.AddCommand(&cobra.Command{
		Use:                "verify [flags]",
		Short:              "Verify the results of an E2E scenario",
		DisableFlagParsing: true,
		RunE: func(c *cobra.Command, args []string) error {
			return runVerifyFunc(args)
		},
	})

	e2eCmd.AddCommand(&cobra.Command{
		Use:                "cleanup [flags]",
		Short:              "Cleanup resources (Helm release, Jira tickets)",
		DisableFlagParsing: true,
		RunE: func(c *cobra.Command, args []string) error {
			return runCleanupFunc(args)
		},
	})

	return e2eCmd
}
