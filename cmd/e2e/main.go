package main

import (
	"fmt"
	"os"

	"recac/pkg/e2e/cmd"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: recac-e2e <command> [args]")
		fmt.Println("Commands: setup, deploy, verify, cleanup, run-all")
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "setup":
		if err := cmd.RunSetup(args); err != nil {
			fmt.Printf("Setup failed: %v\n", err)
			os.Exit(1)
		}
	case "deploy":
		if err := cmd.RunDeploy(args); err != nil {
			fmt.Printf("Deploy failed: %v\n", err)
			os.Exit(1)
		}
	case "wait":
		if err := cmd.RunWait(args); err != nil {
			fmt.Printf("Wait failed: %v\n", err)
			os.Exit(1)
		}
	case "verify":
		if err := cmd.RunVerify(args); err != nil {
			fmt.Printf("Verify failed: %v\n", err)
			os.Exit(1)
		}
	case "cleanup":
		if err := cmd.RunCleanup(args); err != nil {
			fmt.Printf("Cleanup failed: %v\n", err)
			os.Exit(1)
		}
	case "run-all":
		// Call the original monolithic logic (refactored later)
		// For now we might just chain the commands or keep legacy support
		fmt.Println("run-all not yet fully refactored, use legacy runner for now")
		os.Exit(1)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}
