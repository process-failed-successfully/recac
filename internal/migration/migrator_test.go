package migration

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrator(t *testing.T) {
	// 1. Setup DB
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// 2. Setup Temp Dir
	tmpDir := t.TempDir()

	// 3. Create Migrator
	m := NewMigrator(db, tmpDir, "sqlite")

	// 4. Create Migration Files
	// 20230101000000_init
	err = os.WriteFile(filepath.Join(tmpDir, "20230101000000_init.up.sql"), []byte("CREATE TABLE users (id INT);"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "20230101000000_init.down.sql"), []byte("DROP TABLE users;"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 20230102000000_add_email
	// Note: SQLite supports ADD COLUMN. DROP COLUMN support depends on version but usually works in modernc.
	err = os.WriteFile(filepath.Join(tmpDir, "20230102000000_add_email.up.sql"), []byte("ALTER TABLE users ADD COLUMN email TEXT;"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "20230102000000_add_email.down.sql"), []byte("ALTER TABLE users DROP COLUMN email;"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 5. Test Status (Empty)
	status, err := m.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if len(status.Applied) != 0 {
		t.Errorf("Expected 0 applied, got %d", len(status.Applied))
	}
	if len(status.Pending) != 2 {
		t.Errorf("Expected 2 pending, got %d", len(status.Pending))
	}

	// 6. Test Up (All)
	if err := m.Up(0); err != nil {
		t.Fatalf("Up(0) failed: %v", err)
	}

	// Verify DB state
	if _, err := db.Exec("SELECT email FROM users"); err != nil {
		t.Errorf("DB check failed (users table or email column missing): %v", err)
	}

	status, err = m.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Applied) != 2 {
		t.Errorf("Expected 2 applied, got %d", len(status.Applied))
	}
	if len(status.Pending) != 0 {
		t.Errorf("Expected 0 pending, got %d", len(status.Pending))
	}

	// 7. Test Down (1 step)
	if err := m.Down(1); err != nil {
		t.Fatalf("Down(1) failed: %v", err)
	}

	// Verify DB state (email should be gone)
	if _, err := db.Exec("SELECT email FROM users"); err == nil {
		t.Error("Expected error selecting email after rollback, got nil")
	}

	status, err = m.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Applied) != 1 {
		t.Errorf("Expected 1 applied, got %d", len(status.Applied))
	}
	if len(status.Pending) != 1 {
		t.Errorf("Expected 1 pending, got %d", len(status.Pending))
	}
	if status.Pending[0].Version != "20230102000000" {
		t.Errorf("Expected pending version 20230102000000, got %s", status.Pending[0].Version)
	}

	// 8. Test Down (All)
	if err := m.Down(0); err != nil {
		t.Fatalf("Down(0) failed: %v", err)
	}

	// Users table should be gone
	if _, err := db.Exec("SELECT * FROM users"); err == nil {
		t.Error("Expected error selecting from users after full rollback, got nil")
	}
}

func TestGenerate(t *testing.T) {
	tmpDir := t.TempDir()
	// Test without AI
	ver, err := Generate(tmpDir, "add_users", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	files, _ := os.ReadDir(tmpDir)
	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}

	// Check filenames
	hasUp := false
	hasDown := false
	for _, f := range files {
		if strings.Contains(f.Name(), ver) && strings.Contains(f.Name(), "add_users") {
			if strings.HasSuffix(f.Name(), ".up.sql") {
				hasUp = true
			}
			if strings.HasSuffix(f.Name(), ".down.sql") {
				hasDown = true
			}
		}
	}
	if !hasUp || !hasDown {
		t.Error("Missing up or down file")
	}
}
