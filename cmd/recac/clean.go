package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cleanCmd)
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove temporary files created during the session",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Cleaning up temporary files...")

		tempFilesPath := "temp_files.txt"
		file, err := os.Open(tempFilesPath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No temporary files to clean.")
				return
			}
			fmt.Printf("Error opening %s: %v\n", tempFilesPath, err)
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		var filesToRemove []string
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				filesToRemove = append(filesToRemove, line)
			}
		}
		file.Close() // Close BEFORE removal

		for _, f := range filesToRemove {
			absPath, err := filepath.Abs(f)
			if err != nil {
				fmt.Printf("Error resolving path %s: %v\n", f, err)
				continue
			}
			err = os.Remove(absPath)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("File %s already gone.\n", absPath)
				} else {
					fmt.Printf("Error removing %s: %v\n", absPath, err)
				}
			} else {
				fmt.Printf("Removed %s\n", absPath)
			}
		}

		// Optionally remove the temp_files.txt itself or truncate it
		os.Remove(tempFilesPath)
		fmt.Println("Cleanup complete.")
	},
}
