package main

import (
	"fmt"
	"os"
	"path/filepath"

	"recac/internal/utils"

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
		lines, err := utils.ReadLines(tempFilesPath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No temporary files to clean.")
				return
			}
			fmt.Printf("Error opening %s: %v\n", tempFilesPath, err)
			return
		}

		var filesToRemove []string
		for _, line := range lines {
			if line != "" {
				filesToRemove = append(filesToRemove, line)
			}
		}

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
