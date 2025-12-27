package main

import (
	"fmt"
	"os"
	"bufio"

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

		for _, f := range filesToRemove {
			err := os.Remove(f)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("File %s already gone.\n", f)
				} else {
					fmt.Printf("Error removing %s: %v\n", f, err)
				}
			} else {
				fmt.Printf("Removed %s\n", f)
			}
		}

		// Optionally remove the temp_files.txt itself or truncate it
		os.Remove(tempFilesPath)
		fmt.Println("Cleanup complete.")
	},
}
