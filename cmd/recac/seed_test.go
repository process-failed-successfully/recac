package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestSeedCmd(t *testing.T) {
	// 1. Setup temporary workspace and DB
	tmpDir, err := os.MkdirTemp("", "recac-seed-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Create a simple table
	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);`)
	require.NoError(t, err)

	// 2. Setup Mock Agent
	mockAgent := agent.NewMockAgent()
	mockSQL := "INSERT INTO users (name) VALUES ('Alice');\nINSERT INTO users (name) VALUES ('Bob');"
	mockAgent.SetResponse(mockSQL)

	// 3. Override agent factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// 4. Run Seed Command
	// Reset flags manually just in case
	seedExecute = false
	seedCount = 5
	seedOutput = ""

	// Use rootCmd to execute, ensuring proper command selection
	rootCmd.SetArgs([]string{"seed", dbPath, "--execute", "--count", "2"})
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)

	err = rootCmd.Execute()
	require.NoError(t, err)

	// 5. Verify DB Content
	rows, err := db.Query("SELECT name FROM users ORDER BY name")
	require.NoError(t, err)
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		require.NoError(t, err)
		names = append(names, name)
	}

	assert.Equal(t, []string{"Alice", "Bob"}, names)
}

func TestSeedCmd_OutputToFile(t *testing.T) {
	// 1. Setup temporary workspace
	tmpDir, err := os.MkdirTemp("", "recac-seed-test-output")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE items (id INT);`)
	require.NoError(t, err)
	db.Close()

	// 2. Mock Agent
	mockAgent := agent.NewMockAgent()
	mockSQL := "INSERT INTO items VALUES (1);"
	mockAgent.SetResponse(mockSQL)

	// 3. Override factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// 4. Run
	outFile := filepath.Join(tmpDir, "seed.sql")

	// Reset flags hack (since it's a global singleton cmd)
	seedExecute = false
	seedOutput = ""

	rootCmd.SetArgs([]string{"seed", dbPath, "--output", outFile})
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)

	err = rootCmd.Execute()
	require.NoError(t, err)

	// 5. Verify File
	require.FileExists(t, outFile)
	content, err := os.ReadFile(outFile)
	require.NoError(t, err)
	assert.Equal(t, mockSQL, string(content))
}
