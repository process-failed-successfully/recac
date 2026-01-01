package main

import (
	"fmt"
	"os"
	"path/filepath"

	"recac/internal/db"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	signalCmd.AddCommand(clearSignalCmd)
	signalCmd.PersistentFlags().String("path", "", "Project path")
	viper.BindPFlag("path", signalCmd.PersistentFlags().Lookup("path"))

	rootCmd.AddCommand(signalCmd)
}

var signalCmd = &cobra.Command{
	Use:   "signal",
	Short: "Manage project signals",
	Long:  `Manage the persistent signals stored in the project's database (e.g., PROJECT_SIGNED_OFF, QA_PASSED).`,
}

var clearSignalCmd = &cobra.Command{
	Use:   "clear [key]",
	Short: "Clear a specific signal",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		projectPath := viper.GetString("path")
		if projectPath == "" {
			wd, err := os.Getwd()
			if err != nil {
				fmt.Printf("Error determining working directory: %v\n", err)
				os.Exit(1)
			}
			projectPath = wd
		}

		dbPath := filepath.Join(projectPath, ".recac.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			fmt.Printf("Error: Database not found at %s. Are you in a project root?\n", dbPath)
			os.Exit(1)
		}

		store, err := db.NewSQLiteStore(dbPath)
		if err != nil {
			fmt.Printf("Error opening database: %v\n", err)
			os.Exit(1)
		}
		defer store.Close()

		if err := store.DeleteSignal(key); err != nil {
			fmt.Printf("Error clearing signal '%s': %v\n", key, err)
			os.Exit(1)
		}

		fmt.Printf("Signal '%s' cleared successfully.\n", key)
	},
}
