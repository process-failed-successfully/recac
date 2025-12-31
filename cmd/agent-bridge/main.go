package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/db"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Initialize DB connection
	// We assume the DB is at .recac.db in the current directory or workspace root
	// Since agents run in /workspace, and DB is at /workspace/.recac.db
	dbPath := ".recac.db"
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	var cmdErr error

	switch command {
	case "blocker":
		if len(os.Args) < 3 {
			fmt.Println("Usage: agent-bridge blocker <message>")
			os.Exit(1)
		}
		message := os.Args[2]
		cmdErr = store.SetSignal("BLOCKER", message)
		if cmdErr == nil {
			fmt.Println("Blocker signal set.")
		}

	case "qa":
		cmdErr = store.SetSignal("TRIGGER_QA", "true")
		if cmdErr == nil {
			fmt.Println("QA trigger signal set.")
		}

	case "manager":
		cmdErr = store.SetSignal("TRIGGER_MANAGER", "true")
		if cmdErr == nil {
			fmt.Println("Manager trigger signal set.")
		}

	case "verify":
		if len(os.Args) < 4 {
			fmt.Println("Usage: agent-bridge verify <id> <pass/fail>")
			os.Exit(1)
		}
		id := os.Args[2]
		status := os.Args[3]

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
						msg, _ := store.GetSignal("BLOCKER")
						if strings.Contains(msg, "UI Verification Required") {
							store.DeleteSignal("BLOCKER")
							fmt.Println("All UI verifications complete. Clearing blocker.")
						}
					}
				} else {
					fmt.Printf("Error: Feature ID %s not found in %s\n", id, uiPath)
				}
			}
		} else {
			fmt.Printf("Error: Could not read %s\n", uiPath)
		}

	case "signal":
		if len(os.Args) < 4 {
			fmt.Println("Usage: agent-bridge signal <key> <value>")
			os.Exit(1)
		}
		cmdErr = store.SetSignal(os.Args[2], os.Args[3])
		if cmdErr == nil {
			fmt.Printf("Signal %s set to %s.\n", os.Args[2], os.Args[3])
		}

	case "feature":
		if len(os.Args) < 5 || os.Args[2] != "set" {
			fmt.Println("Usage: agent-bridge feature set <id> --status <status> --passes <true/false>")
			os.Exit(1)
		}
		id := os.Args[3]
		var status string
		var passes bool
		for i := 4; i < len(os.Args); i++ {
			if os.Args[i] == "--status" && i+1 < len(os.Args) {
				status = os.Args[i+1]
				i++
			} else if os.Args[i] == "--passes" && i+1 < len(os.Args) {
				passes = os.Args[i+1] == "true"
				i++
			}
		}
		cmdErr = store.UpdateFeatureStatus(id, status, passes)
		if cmdErr == nil {
			fmt.Printf("Feature %s updated: status=%s, passes=%v\n", id, status, passes)
		}

	default:
		printUsage()
		os.Exit(1)
	}

	if cmdErr != nil {
		fmt.Fprintf(os.Stderr, "Error executing command: %v\n", cmdErr)
		os.Exit(1)
	}
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
}
