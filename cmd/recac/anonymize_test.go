package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	_ "modernc.org/sqlite"
)

type mockAnonymizeAgent struct {
	response string
}

func (m *mockAnonymizeAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.response, nil
}

func (m *mockAnonymizeAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.response, nil
}

func TestAnonymizeCmd(t *testing.T) {
	// Create temporary directory for database
	tmpDir, err := os.MkdirTemp("", "recac-test-anonymize")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Create SQLite database and seed with data
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			name TEXT,
			email TEXT
		);
		INSERT INTO users (id, name, email) VALUES (1, 'John Doe', 'john@example.com');
		INSERT INTO users (id, name, email) VALUES (2, 'Jane Smith', 'jane@example.com');
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Override agent factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &mockAnonymizeAgent{
			response: "UPDATE users SET email = 'anon_' || id || '@test.com', name = 'User ' || id;",
		}, nil
	}

	// Set flags for the command
	anonymizeDbPath = dbPath
	anonymizeExecute = true
	anonymizeOutput = "" // Don't write to file

	// Run the command
	// We call runAnonymize directly to avoid Cobra parsing issues in test
	err = runAnonymize(anonymizeCmd, []string{})
	if err != nil {
		t.Fatalf("runAnonymize failed: %v", err)
	}

	// Verify the database content
	rows, err := db.Query("SELECT id, name, email FROM users ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var id int
	var name, email string

	// Check first user
	if !rows.Next() {
		t.Fatal("expected row 1")
	}
	if err := rows.Scan(&id, &name, &email); err != nil {
		t.Fatal(err)
	}
	if name != "User 1" {
		t.Errorf("expected name 'User 1', got '%s'", name)
	}
	if email != "anon_1@test.com" {
		t.Errorf("expected email 'anon_1@test.com', got '%s'", email)
	}

	// Check second user
	if !rows.Next() {
		t.Fatal("expected row 2")
	}
	if err := rows.Scan(&id, &name, &email); err != nil {
		t.Fatal(err)
	}
	if name != "User 2" {
		t.Errorf("expected name 'User 2', got '%s'", name)
	}
	if email != "anon_2@test.com" {
		t.Errorf("expected email 'anon_2@test.com', got '%s'", email)
	}
}
