package main

import (
	"bufio"
	"fmt"
	"os"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(attachCmd)
}

var attachCmd = &cobra.Command{
	Use:   "attach [session-name]",
	Short: "Re-attach to a running session",
	Long:  `Re-attach to a running session to view its output in real-time.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sessionName := args[0]

		sm, err := runner.NewSessionManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create session manager: %v\n", err)
			os.Exit(1)
		}

		session, err := sm.LoadSession(sessionName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: session not found: %v\n", err)
			os.Exit(1)
		}

		if session.Status != "running" {
			fmt.Fprintf(os.Stderr, "Error: session '%s' is not running (status: %s)\n", sessionName, session.Status)
			os.Exit(1)
		}

		fmt.Printf("Attaching to session '%s' (PID: %d)\n", sessionName, session.PID)
		fmt.Println("Press Ctrl+C to detach")
		fmt.Println("===========================================")

		// Stream logs
		logFile, err := sm.GetSessionLogs(sessionName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		file, err := os.Open(logFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to open log file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		// Read and display existing logs
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}

		// Note: Real-time following would require file watching
		// For now, we just show the current logs
		fmt.Println("\n(Real-time following not yet implemented - showing current logs)")
		fmt.Println("Use 'recac-app logs --follow' for continuous updates")
	},
}
