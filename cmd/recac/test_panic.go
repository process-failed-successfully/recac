package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	// This is a hidden command for testing panic recovery
	testPanicCmd.Hidden = true
	rootCmd.AddCommand(testPanicCmd)
}

var testPanicCmd = &cobra.Command{
	Use:   "test-panic",
	Short: "Test panic recovery (hidden command)",
	Long:  `This command simulates a critical panic to test graceful shutdown.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Simulating critical panic...")
		panic("CRITICAL ERROR: Test panic for graceful shutdown verification")
	},
}
