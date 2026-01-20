package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"recac/internal/db"
	"strings"
)

func main() {
	// Check env vars for overrides
	dbType := os.Getenv("RECAC_DB_TYPE")
	dbURL := os.Getenv("RECAC_DB_URL")

	if dbType == "" {
		dbType = "sqlite"
		if dbURL == "" {
			dbURL = ".recac.db"
		}
	}

	projectID := os.Getenv("RECAC_PROJECT_ID")
	if projectID == "" {
		projectID = "default"
	}

	config := db.StoreConfig{
		Type:             dbType,
		ConnectionString: dbURL,
	}

	if err := run(os.Args, config, projectID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, config db.StoreConfig, projectID string) error {
	if len(args) < 2 {
		printUsage()
		return fmt.Errorf("missing command")
	}

	command := args[1]

	// Initialize DB connection
	store, err := db.NewStore(config)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer store.Close()
	var cmdErr error

	switch command {
	case "clear-signal":
		if len(args) < 3 {
			return fmt.Errorf("usage: agent-bridge clear-signal <key>")
		}
		key := args[2]

		// Temporarily use default project ID for clearing signals
		// This logic seems to be intended for a different context or command,
		// but integrating it as requested.
		projectPath, err := os.Getwd() // Assuming current directory is project root
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		dbPath := filepath.Join(projectPath, ".recac.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("Error: Database not found at %s. Are you in a project root?", dbPath)
		}

		projectName := filepath.Base(projectPath)
		if projectName == "." || projectName == "/" {
			cwd, _ := os.Getwd()
			projectName = filepath.Base(cwd)
		}

		// Re-initialize store for SQLite specifically, as per the snippet's intent
		// This might override the 'config' passed to 'run' if it's not SQLite
		sqliteStore, err := db.NewSQLiteStore(dbPath)
		if err != nil {
			return fmt.Errorf("Error opening database: %v", err)
		}
		defer sqliteStore.Close()

		if err := sqliteStore.DeleteSignal(projectName, key); err != nil {
			return fmt.Errorf("Error clearing signal '%s': %v", key, err)
		}
		fmt.Printf("Signal '%s' cleared for project '%s'.\n", key, projectName)

	case "blocker":
		if len(args) < 3 {
			return fmt.Errorf("usage: agent-bridge blocker <message>")
		}
		message := args[2]
		cmdErr = store.SetSignal(projectID, "BLOCKER", message)
		if cmdErr == nil {
			fmt.Println("Blocker signal set.")
		}

	case "qa":
		cmdErr = store.SetSignal(projectID, "TRIGGER_QA", "true")
		if cmdErr == nil {
			fmt.Println("QA trigger signal set.")
		}

	case "manager":
		cmdErr = store.SetSignal(projectID, "TRIGGER_MANAGER", "true")
		if cmdErr == nil {
			fmt.Println("Manager trigger signal set.")
		}

	case "verify":
		if len(args) < 4 {
			return fmt.Errorf("usage: agent-bridge verify <id> <pass/fail>")
		}
		id := args[2]
		status := args[3]

		// 1. Update ui_verification.json
		uiPath := "ui_verification.json"
		data, err := os.ReadFile(uiPath)
		if err == nil {
			var uiReq struct {
				Requests []struct {
					FeatureID   string `json:"feature_id"`
					Instruction string `json:"instruction"`
					Status      string `json:"status"`
					Feedback    string `json:"feedback"`
				} `json:"requests"`
			}
			if err := json.Unmarshal(data, &uiReq); err == nil {
				found := false
				allDone := true
				for i, r := range uiReq.Requests {
					if r.FeatureID == id {
						uiReq.Requests[i].Status = status
						found = true
					}
					if uiReq.Requests[i].Status == "pending_human" {
						allDone = false
					}
				}
				if found {
					updated, _ := json.MarshalIndent(uiReq, "", "  ")
					os.WriteFile(uiPath, updated, 0644)
					fmt.Printf("UI verification for %s updated to %s.\n", id, status)

					if allDone {
						// Optionally clear blocker if it was a UI blocker
						msg, _ := store.GetSignal(projectID, "BLOCKER")
						if strings.Contains(msg, "UI Verification Required") {
							store.DeleteSignal(projectID, "BLOCKER")
							fmt.Println("All UI verifications complete. Clearing blocker.")
						}
					}
				} else {
					return fmt.Errorf("feature ID %s not found in %s", id, uiPath)
				}
			}
		} else {
			return fmt.Errorf("could not read %s", uiPath)
		}

	case "signal":
		if len(args) < 4 {
			return fmt.Errorf("usage: agent-bridge signal <key> <value>")
		}
		key := args[2]
		value := args[3]

		// PROTECT PRIVILEGED SIGNALS
		privilegedSignals := map[string]bool{
			"PROJECT_SIGNED_OFF": true,
			"TRIGGER_QA":         true,
			"TRIGGER_MANAGER":    true,
		}
		if privilegedSignals[key] {
			return fmt.Errorf("signal '%s' is privileged and cannot be set via agent-bridge", key)
		}

		cmdErr = store.SetSignal(projectID, key, value)
		if cmdErr == nil {
			fmt.Printf("Signal %s set to %s.\n", key, value)
		}

	case "feature":
		if len(args) < 3 {
			return fmt.Errorf("usage: agent-bridge feature <set|list> [args]")
		}
		subCmd := args[2]

		if subCmd == "list" {
			// Usage: agent-bridge feature list [--json]
			// We always return JSON for now as it's the efficient format
			content, err := store.GetFeatures(projectID)
			if err != nil {
				return fmt.Errorf("failed to get features: %w", err)
			}
			if content == "" {
				// Return empty feature list structure
				content = `{"features":[]}`
			}
			fmt.Println(content)
			return nil
		}

		if subCmd == "set" {
			if len(args) < 5 {
				return fmt.Errorf("usage: agent-bridge feature set <id> --status <status> --passes <true/false>")
			}
			id := args[3]
			var status string
			var passes bool
			for i := 4; i < len(args); i++ {
				if args[i] == "--status" && i+1 < len(args) {
					status = args[i+1]
					i++
				} else if args[i] == "--passes" && i+1 < len(args) {
					passes = args[i+1] == "true"
					i++
				}
			}
			cmdErr = store.UpdateFeatureStatus(projectID, id, status, passes)

			if cmdErr == nil {
				fmt.Printf("Feature %s updated: status=%s, passes=%v\n", id, status, passes)

				// Auto-Completion Check
				// If all features are done/passed, signal completion automatically.
				content, err := store.GetFeatures(projectID)
				if err == nil && content != "" {
					var fl db.FeatureList
					if json.Unmarshal([]byte(content), &fl) == nil {
						allDone := true
						for _, f := range fl.Features {
							if strings.ToLower(f.Status) != "done" || !f.Passes {
								allDone = false
								break
							}
						}
						if allDone {
							fmt.Println("All features completed and passed. Auto-signaling COMPLETED.")
							if err := store.SetSignal(projectID, "COMPLETED", "true"); err != nil {
								fmt.Printf("Warning: Failed to set COMPLETED signal: %v\n", err)
							}
						}
					}
				}
			}
		} else {
			return fmt.Errorf("unknown feature subcommand: %s", subCmd)
		}

	case "import":
		// Usage: cat features.json | agent-bridge import
		// Reads JSON from Stdin and saves to DB
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}

		if len(data) == 0 {
			return fmt.Errorf("empty input")
		}

		// Validate JSON
		var fl db.FeatureList
		if err := json.Unmarshal(data, &fl); err != nil {
			return fmt.Errorf("invalid json: %w", err)
		}

		if err := store.SaveFeatures(projectID, string(data)); err != nil {
			return fmt.Errorf("failed to save features to DB: %w", err)
		}
		fmt.Printf("Successfully imported %d features.\n", len(fl.Features))

	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", command)
	}

	if cmdErr != nil {
		return fmt.Errorf("error executing command: %w", cmdErr)
	}
	return nil
}

func printUsage() {
	fmt.Println("Usage: agent-bridge <command> [arguments]")
	fmt.Println("Commands:")
	fmt.Println("  blocker <message>      Set a blocker signal")
	fmt.Println("  qa                     Trigger QA process")
	fmt.Println("  manager                Trigger Manager review")
	fmt.Println("  verify <id> <pass/fail> Update UI verification request")
	fmt.Println("  signal <key> <value>   Set a generic signal")
	fmt.Println("  feature set <id> --status <status> --passes <true/false> Update feature status")
	fmt.Println("  feature list           List features (JSON)")
}
