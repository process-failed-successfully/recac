package main

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	_ "modernc.org/sqlite"
)

func TestSeedCmd(t *testing.T) {
	// 1. Setup temporary DB
	tmpDB := "test_seed.db"
	defer os.Remove(tmpDB)

	db, err := sql.Open("sqlite", tmpDB)
	assert.NoError(t, err)
	defer db.Close()

	// Create schema
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			name TEXT
		);
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY,
			user_id INTEGER,
			title TEXT,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);
	`)
	assert.NoError(t, err)
	db.Close() // Close so seedCmd can open it

	// 2. Mock Agent Factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAg := agent.NewMockAgent()
	mockAg.SetResponse(`
INSERT INTO users (name) VALUES ('Alice');
INSERT INTO users (name) VALUES ('Bob');
INSERT INTO posts (user_id, title) VALUES (1, 'First Post');
INSERT INTO posts (user_id, title) VALUES (2, 'Second Post');
`)

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}

	// 3. Run Seed Command
	// Save original flags
	origDb := seedDbPath
	origRows := seedRows
	origExec := seedExecute
	origOut := seedOutput

	defer func() {
		seedDbPath = origDb
		seedRows = origRows
		seedExecute = origExec
		seedOutput = origOut
	}()

	seedDbPath = tmpDB
	seedRows = 2
	seedExecute = true
	seedOutput = ""

	// We can pass empty args because we set flags globally
	err = runSeed(seedCmd, []string{})
	assert.NoError(t, err)

	// 4. Verify Data
	db, err = sql.Open("sqlite", tmpDB)
	assert.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	err = db.QueryRow("SELECT COUNT(*) FROM posts").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestSeedCmd_NoDBPath(t *testing.T) {
	// Setup: Run in empty directory
	tmpDir := t.TempDir()
	oldwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldwd)

	origDb := seedDbPath
	defer func() { seedDbPath = origDb }()
	seedDbPath = ""

	err := runSeed(seedCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection string or file path required")
}

func TestSeedCmd_ExecuteFailure(t *testing.T) {
	// Test execution failure due to invalid SQL syntax
	tmpDB := "test_seed_fail.db"
	defer os.Remove(tmpDB)

	db, err := sql.Open("sqlite", tmpDB)
	assert.NoError(t, err)

	// Ensure file is created
	_, err = db.Exec("CREATE TABLE users (id INTEGER);")
	assert.NoError(t, err)
	db.Close()

	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAg := agent.NewMockAgent()
	mockAg.SetResponse("THIS IS INVALID SQL")

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}

	origDb := seedDbPath
	origExec := seedExecute
	defer func() {
		seedDbPath = origDb
		seedExecute = origExec
	}()

	seedDbPath = tmpDB
	seedExecute = true

	err = runSeed(seedCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execution failed")
}
