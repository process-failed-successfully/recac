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
	signalCmd.AddCommand(setSignalCmd)
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
				exit(1)
			}
			projectPath = wd
		}

		dbPath := filepath.Join(projectPath, ".recac.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			fmt.Printf("Error: Database not found at %s. Are you in a project root?\n", dbPath)
			exit(1)
		}

		projectName := filepath.Base(projectPath)
		if projectName == "." || projectName == "/" {
			cwd, _ := os.Getwd()
			projectName = filepath.Base(cwd)
		}

		store, err := db.NewSQLiteStore(dbPath)
		if err != nil {
			fmt.Printf("Error opening database: %v\n", err)
			exit(1)
		}
		defer store.Close()

		if err := store.DeleteSignal(projectName, key); err != nil {
			fmt.Printf("Error clearing signal '%s': %v\n", key, err)
			exit(1)
		}

		fmt.Printf("Signal '%s' cleared successfully.\n", key)
	},
}

var setSignalCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a specific signal",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := args[1]
		projectPath := viper.GetString("path")
		if projectPath == "" {
			wd, err := os.Getwd()
			if err != nil {
				fmt.Printf("Error determining working directory: %v\n", err)
				exit(1)
			}
			projectPath = wd
		}

		// Support Postgres via Env like Session does
		dbType := os.Getenv("RECAC_DB_TYPE")
		dbURL := os.Getenv("RECAC_DB_URL")

		var store db.Store
		var err error

		if dbType == "postgres" && dbURL != "" {
			store, err = db.NewPostgresStore(dbURL)
		} else {
			// Fallback to SQLite
			dbPath := filepath.Join(projectPath, ".recac.db")
			if _, err := os.Stat(dbPath); os.IsNotExist(err) && dbType == "" {
				// Only complain if explicit type wasn't provided
				fmt.Printf("Error: Database not found at %s. Are you in a project root?\n", dbPath)
				exit(1)
			}
			store, err = db.NewSQLiteStore(dbPath)
		}

		if err != nil {
			fmt.Printf("Error opening database: %v\n", err)
			exit(1)
		}
		defer store.Close()

		// Derive project name (or use env)
		projectName := os.Getenv("RECAC_PROJECT_ID")
		if projectName == "" {
			projectName = filepath.Base(projectPath)
			if projectName == "." || projectName == "/" {
				cwd, _ := os.Getwd()
				projectName = filepath.Base(cwd)
			}
		}

		if err := store.SetSignal(projectName, key, value); err != nil {
			fmt.Printf("Error setting signal '%s': %v\n", key, err)
			exit(1)
		}

		fmt.Printf("Signal '%s' set to '%s' successfully for project '%s'.\n", key, value, projectName)
	},
}
