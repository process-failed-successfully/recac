package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	_ "modernc.org/sqlite"
)

func TestSeedCmd(t *testing.T) {
	// 1. Create Temp Dir & DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// Create Schema
	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// 2. Mock Agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockResponse := "INSERT INTO users (id, name) VALUES (1, 'Test User');"

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		m := agent.NewMockAgent()
		m.SetResponse(mockResponse)
		return m, nil
	}

	// 3. Configure & Run Command
	// Reset flags/vars
	seedDbPath = dbPath
	seedExecute = true
	seedCount = 1
	seedClean = false
	seedTables = nil
	seedOutput = ""

	// We pass empty args, runSeed will use seedDbPath
	err = runSeed(seedCmd, []string{})
	if err != nil {
		t.Fatalf("seed command failed: %v", err)
	}

	// 4. Verify Data
	// Re-open DB to verify persistence (though shared mode should work)
	// We use the existing connection
	row := db.QueryRow("SELECT name FROM users WHERE id = 1")
	var name string
	err = row.Scan(&name)
	if err != nil {
		t.Fatalf("failed to query inserted data: %v", err)
	}

	if name != "Test User" {
		t.Errorf("expected 'Test User', got '%s'", name)
	}
}

func TestSeedCmd_FilterTables(t *testing.T) {
	// 1. Create Temp Dir & DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_filter.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// Create Schema with 2 tables
	_, err = db.Exec(`
		CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);
		CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	// 2. Mock Agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	// Agent should only receive schema for 'users' if filtered
	// We can't easily assert the prompt content with the current mock,
	// but we can verify it runs successfully.

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		m := agent.NewMockAgent()
		m.SetResponse("INSERT INTO users (id, name) VALUES (2, 'Filter User');")
		return m, nil
	}

	// 3. Configure & Run Command
	seedDbPath = dbPath
	seedExecute = true
	seedCount = 1
	seedTables = []string{"users"} // Filter for users only
	seedClean = false

	err = runSeed(seedCmd, []string{})
	if err != nil {
		t.Fatalf("seed command failed: %v", err)
	}

	// 4. Verify Data
	var name string
	err = db.QueryRow("SELECT name FROM users WHERE id = 2").Scan(&name)
	if err != nil {
		t.Fatalf("failed to query user: %v", err)
	}
	if name != "Filter User" {
		t.Errorf("expected 'Filter User', got '%s'", name)
	}
}
