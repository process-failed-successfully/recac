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

// MockSQLAgent implements agent.Agent for testing
type MockSQLAgent struct {
	Response string
	Err      error
}

func (m *MockSQLAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func (m *MockSQLAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, m.Err
}

func TestSQLCommand_Generation(t *testing.T) {
	// 1. Setup DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE users (id INT, name TEXT);")
	require.NoError(t, err)
	db.Close()

	// 2. Mock Agent
	mockAgent := &MockSQLAgent{
		Response: "SELECT * FROM users;",
	}

	// Override factory
	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = oldFactory }()

	// 3. Run Command
	output, err := executeCommand(rootCmd, "sql", "show users", "--db", dbPath)
	require.NoError(t, err)

	assert.Contains(t, output, "SELECT * FROM users;")
}

func TestSQLCommand_Execution(t *testing.T) {
	// 1. Setup DB with data
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE users (id INT, name TEXT);")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO users VALUES (1, 'Alice');")
	require.NoError(t, err)
	db.Close()

	// 2. Mock Agent
	mockAgent := &MockSQLAgent{
		Response: "SELECT name FROM users WHERE id=1;",
	}

	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = oldFactory }()

	// 3. Run Command with --execute
	output, err := executeCommand(rootCmd, "sql", "who is user 1", "--db", dbPath, "--execute")
	require.NoError(t, err)

	assert.Contains(t, output, "Generated SQL:")
	assert.Contains(t, output, "Alice")
	assert.Contains(t, output, "name") // header
}

func TestSQLCommand_Output(t *testing.T) {
    // 1. Setup DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE t (c INT);")
	require.NoError(t, err)
	db.Close()

    outPath := filepath.Join(tmpDir, "out.sql")

	// 2. Mock Agent
	mockAgent := &MockSQLAgent{
		Response: "SELECT * FROM t;",
	}

    oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = oldFactory }()

    // 3. Run
    output, err := executeCommand(rootCmd, "sql", "q", "--db", dbPath, "--output", outPath)
    require.NoError(t, err)

    assert.Contains(t, output, "SQL saved to")

    content, err := os.ReadFile(outPath)
    require.NoError(t, err)
    assert.Equal(t, "SELECT * FROM t;", string(content))
}
