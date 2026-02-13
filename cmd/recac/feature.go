package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/db"
	"recac/internal/git"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var getStoreFunc = defaultGetStore

func defaultGetStore() (db.Store, error) {
	dbType := viper.GetString("db.type")
	if dbType == "" {
		dbType = os.Getenv("RECAC_DB_TYPE")
	}
	// Default to sqlite
	if dbType == "" {
		dbType = "sqlite"
	}

	dbURL := viper.GetString("db.url")
	if dbURL == "" {
		dbURL = os.Getenv("RECAC_DB_URL")
	}
	// Default to .recac.db
	if dbURL == "" {
		dbURL = ".recac.db"
	}

	config := db.StoreConfig{
		Type:             dbType,
		ConnectionString: dbURL,
	}

	return db.NewStore(config)
}

func init() {
	featureCmd.AddCommand(featureStartCmd)
	featureCmd.AddCommand(featureListCmd)
	featureCmd.AddCommand(featureStatusCmd)
	featureCmd.AddCommand(featureImportCmd)
	rootCmd.AddCommand(featureCmd)
}

var featureCmd = &cobra.Command{
	Use:   "feature",
	Short: "Manage features",
}

var featureStartCmd = &cobra.Command{
	Use:   "start [name]",
	Short: "Start a new feature",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		branchName := fmt.Sprintf("feature/%s", name)

		fmt.Printf("Starting feature: %s\n", name)
		fmt.Printf("Creating branch: %s\n", branchName)

		err := git.NewClient().CheckoutNewBranch("", branchName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			exit(1)
		}

		fmt.Printf("Successfully switched to branch %s\n", branchName)
	},
}

var featureListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all features",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getStoreFunc()
		if err != nil {
			return fmt.Errorf("failed to connect to DB: %w", err)
		}
		defer store.Close()

		projectID := viper.GetString("project_id")
		if projectID == "" {
			projectID = os.Getenv("RECAC_PROJECT_ID")
		}
		if projectID == "" {
			projectID = "default"
		}

		content, err := store.GetFeatures(projectID)
		if err != nil {
			return fmt.Errorf("failed to get features: %w", err)
		}

		if content == "" {
			cmd.Println("No features found.")
			return nil
		}

		var fl db.FeatureList
		if err := json.Unmarshal([]byte(content), &fl); err != nil {
			return fmt.Errorf("failed to parse features: %w", err)
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tCATEGORY\tPRIORITY\tSTATUS\tPASSES\tDESCRIPTION")
		for _, f := range fl.Features {
			desc := f.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\t%s\n",
				f.ID, f.Category, f.Priority, f.Status, f.Passes, desc)
		}
		w.Flush()
		return nil
	},
}

var featureStatusCmd = &cobra.Command{
	Use:   "status <id> <status> [pass/fail]",
	Short: "Update status of a feature",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		status := args[1]
		var passes *bool

		if len(args) > 2 {
			p := strings.ToLower(args[2]) == "true" || strings.ToLower(args[2]) == "pass"
			passes = &p
		}

		store, err := getStoreFunc()
		if err != nil {
			return fmt.Errorf("failed to connect to DB: %w", err)
		}
		defer store.Close()

		projectID := viper.GetString("project_id")
		if projectID == "" {
			projectID = os.Getenv("RECAC_PROJECT_ID")
		}
		if projectID == "" {
			projectID = "default"
		}

		// 1. Get current state to preserve 'passes' if not provided
		content, err := store.GetFeatures(projectID)
		if err != nil {
			return fmt.Errorf("failed to get features: %w", err)
		}

		if content == "" {
			return fmt.Errorf("no features found")
		}

		var fl db.FeatureList
		if err := json.Unmarshal([]byte(content), &fl); err != nil {
			return fmt.Errorf("failed to parse features: %w", err)
		}

		var currentPasses bool
		found := false
		for _, f := range fl.Features {
			if f.ID == id {
				currentPasses = f.Passes
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("feature %s not found", id)
		}

		newPasses := currentPasses
		if passes != nil {
			newPasses = *passes
		}

		// 2. Update
		if err := store.UpdateFeatureStatus(projectID, id, status, newPasses); err != nil {
			return fmt.Errorf("failed to update feature status: %w", err)
		}

		cmd.Printf("Feature %s updated: status=%s, passes=%v\n", id, status, newPasses)
		return nil
	},
}

var featureImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import features from a JSON file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Validate JSON
		var fl db.FeatureList
		if err := json.Unmarshal(data, &fl); err != nil {
			return fmt.Errorf("invalid json: %w", err)
		}

		store, err := getStoreFunc()
		if err != nil {
			return fmt.Errorf("failed to connect to DB: %w", err)
		}
		defer store.Close()

		projectID := viper.GetString("project_id")
		if projectID == "" {
			projectID = os.Getenv("RECAC_PROJECT_ID")
		}
		if projectID == "" {
			projectID = "default"
		}

		if err := store.SaveFeatures(projectID, string(data)); err != nil {
			return fmt.Errorf("failed to save features to DB: %w", err)
		}

		cmd.Printf("Successfully imported %d features.\n", len(fl.Features))
		return nil
	},
}
