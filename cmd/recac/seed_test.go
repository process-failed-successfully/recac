package main

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
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

	// 3. Run Command Directly
	// Set global flags manually
	seedTable = "users"
	seedCount = 2
	seedYes = true // Skip confirmation prompt

	// Create dummy command
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// cmd.SetContext is not needed if we don't use it, but runSeed might use cmd.Context()
	// runSeed uses cmd.Context() for agentClientFactory and execInsert.
	// Default cmd context is background if not set? No, likely nil.
	cmd.SetContext(context.Background())

	// Call runSeed directly
	args := []string{dbPath}
	err = runSeed(cmd, args)

	// Reset flags to defaults to avoid polluting other tests
	seedTable = ""
	seedCount = 10
	seedYes = false

	require.NoError(t, err, "runSeed failed: %s", buf.String())

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
