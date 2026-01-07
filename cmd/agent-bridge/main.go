package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/db"
	"strconv"
	"strings"
)

func main() {
	if err := run(os.Args, ".recac.db"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, dbPath string) error {
	if len(args) < 2 {
		printUsage()
		return fmt.Errorf("missing command")
	}

	command := args[1]

	// File System commands (DB Independent)
	switch command {
	case "list-files":
		root := "."
		if len(args) > 2 {
			root = args[2]
		}
		files, err := listFiles(root)
		if err != nil {
			return err
		}
		// Output JSON for easy parsing by agents
		jsonBytes, err := json.MarshalIndent(files, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
		return nil

	case "read-file":
		if len(args) < 3 {
			return fmt.Errorf("usage: agent-bridge read-file <path> [--start-line N] [--end-line M]")
		}
		path := args[2]
		startLine := 0
		endLine := 0

		for i := 3; i < len(args); i++ {
			if args[i] == "--start-line" && i+1 < len(args) {
				if v, err := strconv.Atoi(args[i+1]); err == nil {
					startLine = v
					i++
				}
			} else if args[i] == "--end-line" && i+1 < len(args) {
				if v, err := strconv.Atoi(args[i+1]); err == nil {
					endLine = v
					i++
				}
			}
		}

		content, err := readFile(path, startLine, endLine)
		if err != nil {
			return err
		}
		// Output raw content
		fmt.Print(content)
		return nil

	case "search":
		if len(args) < 3 {
			return fmt.Errorf("usage: agent-bridge search <query> [path]")
		}
		query := args[2]
		root := "."
		if len(args) > 3 {
			root = args[3]
		}

		results, err := searchFiles(query, root)
		if err != nil {
			return err
		}
		for _, r := range results {
			fmt.Println(r)
		}
		return nil
	}

	// DB Dependent commands
	// Initialize DB connection
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer store.Close()

	var cmdErr error

	switch command {
	case "blocker":
		if len(args) < 3 {
			return fmt.Errorf("usage: agent-bridge blocker <message>")
		}
		message := args[2]
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
						msg, _ := store.GetSignal("BLOCKER")
						if strings.Contains(msg, "UI Verification Required") {
							store.DeleteSignal("BLOCKER")
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
			"QA_PASSED":          true,
			"COMPLETED":          true,
			"TRIGGER_QA":         true,
			"TRIGGER_MANAGER":    true,
		}

		if privilegedSignals[key] {
			return fmt.Errorf("signal '%s' is privileged and cannot be set via agent-bridge", key)
		}

		cmdErr = store.SetSignal(key, value)
		if cmdErr == nil {
			fmt.Printf("Signal %s set to %s.\n", key, value)
		}

	case "feature":
		if len(args) < 5 || args[2] != "set" {
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
		cmdErr = store.UpdateFeatureStatus(id, status, passes)
		if cmdErr == nil {
			fmt.Printf("Feature %s updated: status=%s, passes=%v\n", id, status, passes)
		}

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
	fmt.Println("  list-files [path]      List files in directory (ignores common artifacts)")
	fmt.Println("  read-file <path>       Read file content")
	fmt.Println("    Flags: --start-line <n> --end-line <n>")
	fmt.Println("  search <query> [path]  Search text in files")
	fmt.Println("  blocker <message>      Set a blocker signal")
	fmt.Println("  qa                     Trigger QA process")
	fmt.Println("  manager                Trigger Manager review")
	fmt.Println("  verify <id> <pass/fail> Update UI verification request")
	fmt.Println("  signal <key> <value>   Set a generic signal")
	fmt.Println("  feature set <id> --status <status> --passes <true/false> Update feature status")
}
