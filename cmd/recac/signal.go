package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"recac/internal/db"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	signalCmd.AddCommand(listSignalCmd)
	signalCmd.AddCommand(setSignalCmd)
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

var listSignalCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active signals",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		store, projectName, err := getStoreAndProjectName()
		if err != nil {
			fmt.Println(err)
			exit(1)
		}
		defer store.Close()

		signals, err := store.ListSignals(projectName)
		if err != nil {
			fmt.Printf("Error listing signals: %v\n", err)
			exit(1)
		}

		if len(signals) == 0 {
			fmt.Println("No active signals found.")
			return
		}

		// Sort keys for consistent output
		keys := make([]string, 0, len(signals))
		for k := range signals {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		fmt.Printf("Active signals for project '%s':\n", projectName)
		for _, k := range keys {
			fmt.Printf("  %s: %s\n", k, signals[k])
		}
	},
}

var setSignalCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a signal value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := args[1]

		store, projectName, err := getStoreAndProjectName()
		if err != nil {
			fmt.Println(err)
			exit(1)
		}
		defer store.Close()

		if err := store.SetSignal(projectName, key, value); err != nil {
			fmt.Printf("Error setting signal '%s': %v\n", key, err)
			exit(1)
		}

		fmt.Printf("Signal '%s' set to '%s'.\n", key, value)
	},
}

var clearSignalCmd = &cobra.Command{
	Use:   "clear [key]",
	Short: "Clear a specific signal",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]

		store, projectName, err := getStoreAndProjectName()
		if err != nil {
			fmt.Println(err)
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

func getStoreAndProjectName() (db.Store, string, error) {
	projectPath := viper.GetString("path")
	if projectPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, "", fmt.Errorf("error determining working directory: %w", err)
		}
		projectPath = wd
	}

	dbPath := filepath.Join(projectPath, ".recac.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("error: Database not found at %s. Are you in a project root?", dbPath)
	}

	projectName := filepath.Base(projectPath)
	if projectName == "." || projectName == "/" {
		cwd, _ := os.Getwd()
		projectName = filepath.Base(cwd)
	}

	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		return nil, "", fmt.Errorf("error opening database: %w", err)
	}

	return store, projectName, nil
}
