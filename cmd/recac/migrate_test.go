package main

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

func TestMigrateCommands(t *testing.T) {
	// 1. Setup Env
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	migDir := filepath.Join(tmpDir, "migrations")

	// Create DB
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.Close() // Close so command can open it

	// 2. Override globals for test
	migrateDB = dbPath
	migrateDir = migDir

	// Helper to run subcommand
	run := func(subCmd *cobra.Command, args ...string) string {
		buf := new(bytes.Buffer)
		subCmd.SetOut(buf)
		subCmd.SetErr(buf)

		// Reset flags if needed (since they are package globals)
		// migrateSteps is used by up/down
		// migrateAI is used by create

		// Manually set args
		// But RunE(cmd, args) expects args without command name
		if err := subCmd.RunE(subCmd, args); err != nil {
			t.Fatalf("Command failed: %v", err)
		}
		return buf.String()
	}

	// 3. Test Create
	migrateAI = ""
	out := run(migrateCreateCmd, "init_users")
	if !strings.Contains(out, "Created migration") {
		t.Errorf("Create failed output: %s", out)
	}

	// Verify files created
	files, err := os.ReadDir(migDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("Expected 2 migration files, got %d", len(files))
	}

	// Overwrite files with valid SQL for testing
	for _, f := range files {
		path := filepath.Join(migDir, f.Name())
		if strings.HasSuffix(f.Name(), ".up.sql") {
			os.WriteFile(path, []byte("CREATE TABLE users (id INT);"), 0644)
		} else {
			os.WriteFile(path, []byte("DROP TABLE users;"), 0644)
		}
	}

	// 4. Test Status (Pending)
	out = run(migrateStatusCmd)
	if !strings.Contains(out, "Pending") {
		t.Errorf("Status should show Pending. Output:\n%s", out)
	}

	// 5. Test Up
	migrateSteps = 0 // All
	out = run(migrateUpCmd)
	if !strings.Contains(out, "up-to-date") {
		t.Errorf("Up failed output: %s", out)
	}

	// Verify DB state
	db, _ = sql.Open("sqlite", dbPath)
	if _, err := db.Exec("SELECT id FROM users"); err != nil {
		t.Errorf("DB check failed (table users should exist): %v", err)
	}
	db.Close()

	// 6. Test Status (Applied)
	out = run(migrateStatusCmd)
	if !strings.Contains(out, "Applied") {
		t.Errorf("Status should show Applied. Output:\n%s", out)
	}

	// 7. Test Down
	migrateSteps = 1
	out = run(migrateDownCmd)
	if !strings.Contains(out, "Rollback complete") {
		t.Errorf("Down failed output: %s", out)
	}

	// Verify DB state
	db, _ = sql.Open("sqlite", dbPath)
	if _, err := db.Exec("SELECT id FROM users"); err == nil {
		t.Error("Table users should be gone after rollback")
	}
	db.Close()
}
