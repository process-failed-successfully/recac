package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	_ "modernc.org/sqlite"
)

// MockAgentForSeed
type MockAgentForSeed struct {
	Response string
}

func (m *MockAgentForSeed) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockAgentForSeed) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestSeedCmd(t *testing.T) {
	// Override factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockResponse := "INSERT INTO users (id, name) VALUES (1, 'Alice');"
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockAgentForSeed{Response: mockResponse}, nil
	}

	// Setup Temp DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create Schema
	_, err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Generate SQL to File", func(t *testing.T) {
		outFile := filepath.Join(tmpDir, "seed.sql")

		// Reset flags
		seedDbStr = dbPath
		seedRows = 5
		seedExecute = false
		seedOutput = outFile
		seedClean = false

		err := runSeed(seedCmd, []string{})
		assert.NoError(t, err)

		content, err := os.ReadFile(outFile)
		assert.NoError(t, err)
		assert.Equal(t, mockResponse, string(content))
	})

	t.Run("Execute SQL", func(t *testing.T) {
		seedDbStr = dbPath
		seedRows = 5
		seedExecute = true
		seedOutput = ""
		seedClean = false

		err := runSeed(seedCmd, []string{})
		assert.NoError(t, err)

		// Verify data inserted
		// We need to re-open DB or ensure 'executeSQL' didn't lock it?
		// executeSQL opens/closes its own connection.
		// SQLite handles concurrency fine usually if WAL or file locking is ok.

		row := db.QueryRow("SELECT name FROM users WHERE id=1")
		var name string
		err = row.Scan(&name)
		assert.NoError(t, err)
		assert.Equal(t, "Alice", name)
	})
}
