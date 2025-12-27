package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

func main() {
	// Recover from any panics in the application
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "\n=== CRITICAL ERROR: Application Panic ===\n")
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", r)
			fmt.Fprintf(os.Stderr, "Stack trace:\n%s\n", debug.Stack())
			fmt.Fprintf(os.Stderr, "Attempting graceful shutdown...\n")
			// Exit with non-zero code to indicate failure
			os.Exit(1)
		}
	}()

	Execute()
}