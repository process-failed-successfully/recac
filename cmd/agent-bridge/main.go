package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/db"
	"strings"
)

func main() {
	// Default DB path, can be overridden if needed but usually static in container
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

	// Commands that don't need DB access
	switch command {
	case "list-files":
		dir := "."
		if len(args) > 2 {
			dir = args[2]
		}
		return listFiles(dir)
	case "search":
		if len(args) < 3 {
			return fmt.Errorf("usage: agent-bridge search <query> [path]")
		}
		query := args[2]
		path := "."
		if len(args) > 3 {
			path = args[3]
		}
		return searchFiles(path, query)
	case "read-file":
		if len(args) < 3 {
			return fmt.Errorf("usage: agent-bridge read-file <path> [start_line] [end_line]")
		}
		fpath := args[2]
		start := 0
		end := -1
		if len(args) > 3 {
			fmt.Sscanf(args[3], "%d", &start)
		}
		if len(args) > 4 {
			fmt.Sscanf(args[4], "%d", &end)
		}
		return readFile(fpath, start, end)
	}

	// Commands that need DB access
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
	fmt.Println("  list-files [dir]       List files (respecting ignores)")
	fmt.Println("  search <query> [dir]   Search text in files (respecting ignores)")
	fmt.Println("  read-file <path> [s] [e] Read file with line numbers (optional start/end lines)")
	fmt.Println("  blocker <message>      Set a blocker signal")
	fmt.Println("  qa                     Trigger QA process")
	fmt.Println("  manager                Trigger Manager review")
	fmt.Println("  verify <id> <pass/fail> Update UI verification request")
	fmt.Println("  signal <key> <value>   Set a generic signal")
	fmt.Println("  feature set <id> --status <status> --passes <true/false> Update feature status")
}

// Helpers for file operations

func isIgnored(name string, isDir bool) bool {
	// Common ignores
	if name == ".git" || name == ".idea" || name == ".vscode" || name == "node_modules" || name == "dist" || name == "build" || name == "vendor" || name == ".recac.db" {
		return true
	}
	if !isDir {
		if strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".sum") || strings.HasSuffix(name, ".lock") {
			return true
		}
	}
	return false
}

func listFiles(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if isIgnored(info.Name(), true) && path != root {
				return filepath.SkipDir
			}
		} else {
			if !isIgnored(info.Name(), false) {
				fmt.Println(path)
			}
		}
		return nil
	})
}

func searchFiles(root, query string) error {
	found := false
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if isIgnored(info.Name(), true) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if isIgnored(info.Name(), false) {
			return nil
		}

		// Read file
		file, err := os.Open(path)
		if err != nil {
			return nil // Skip unreadable
		}
		defer file.Close()

		// Simple line scan
		scanner := bufio.NewScanner(file)
		lineNum := 1
		for scanner.Scan() {
			text := scanner.Text()
			if strings.Contains(text, query) {
				fmt.Printf("%s:%d: %s\n", path, lineNum, strings.TrimSpace(text))
				found = true
			}
			lineNum++
		}
		return nil
	})
	if !found && err == nil {
		fmt.Println("No matches found.")
	}
	return err
}

func readFile(path string, start, end int) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 1
	for scanner.Scan() {
		if lineNum >= start && (end == -1 || lineNum <= end) {
			fmt.Printf("%4d | %s\n", lineNum, scanner.Text())
		}
		if end != -1 && lineNum > end {
			break
		}
		lineNum++
	}
	return scanner.Err()
}
