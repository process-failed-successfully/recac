package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

// SimpleMockAgent implementation for testing
type SimpleMockAgent struct {
	Response string
	Err      error
}

func (m *SimpleMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func (m *SimpleMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, m.Err
}

func TestSeedCmd_SQLite(t *testing.T) {
	// 1. Setup Temp DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Create Table
	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, email TEXT);`)
	require.NoError(t, err)
	db.Close() // Close so seed command can open it

	// 2. Mock Agent Factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	expectedSQL := "INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com');\nINSERT INTO users (name, email) VALUES ('Bob', 'bob@example.com');"

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &SimpleMockAgent{
			Response: expectedSQL,
		}, nil
	}

	// 3. Run Command
	// We use resetFlags helper from test_helpers_test.go if available, or just manually set flags?
	// The helpers are in the same package (main), so they are available.
	// But `resetFlags` is called inside `executeCommand`.
	// I'll try to use `executeCommand` if possible, but `seedCmd` is global.
	// `seedCmd` uses flags bound to global vars `seedTable`, `seedCount`, `seedYes`.
	// I need to reset them manually or trust `executeCommand`'s resetFlags.

	// Since `executeCommand` calls `root.Execute()`, I should call `seedCmd` via `seed`.
	// But `seedCmd` is a subcommand.

	// Let's invoke runSeed directly or via root command.
	// Invoking via root command is safer for flag parsing.

	args := []string{"seed", dbPath, "--table", "users", "--count", "2", "--yes"}

	// Reset global flags manually just in case
	seedTable = ""
	seedCount = 10
	seedYes = false

	output, err := executeCommand(rootCmd, args...)
	require.NoError(t, err, "Command failed: %s", output)

	// 4. Verify DB Content
	db, err = sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	var name string
	err = db.QueryRow("SELECT name FROM users WHERE name='Alice'").Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "Alice", name)
}
