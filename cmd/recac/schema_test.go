package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestExtractSchema_SQLite(t *testing.T) {
	// 1. Create temporary SQLite DB
	tmpDir, err := os.MkdirTemp("", "recac-schema-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// 2. Create Schema
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			username TEXT NOT NULL,
			email TEXT
		);
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY,
			user_id INTEGER,
			title TEXT,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);
	`)
	require.NoError(t, err)
	db.Close() // Close so the command can open it

	// 3. Run extractSchema
	schema, err := extractSchema(dbPath)
	require.NoError(t, err)
	require.NotNil(t, schema)

	// 4. Assertions
	assert.Len(t, schema.Tables, 2)

	// Check Users table
	var userTable *Table
	for _, t := range schema.Tables {
		if t.Name == "users" {
			userTable = &t
			break
		}
	}
	require.NotNil(t, userTable)
	assert.Equal(t, "users", userTable.Name)
	assert.Len(t, userTable.Columns, 3) // id, username, email

	// Check Posts table
	var postTable *Table
	for _, t := range schema.Tables {
		if t.Name == "posts" {
			postTable = &t
			break
		}
	}
	require.NotNil(t, postTable)
	assert.Equal(t, "posts", postTable.Name)
	assert.Len(t, postTable.Columns, 3) // id, user_id, title

	// Check FK
	require.Len(t, postTable.ForeignKeys, 1)
	assert.Equal(t, "user_id", postTable.ForeignKeys[0].FromColumn)
	assert.Equal(t, "users", postTable.ForeignKeys[0].ToTable)
	assert.Equal(t, "id", postTable.ForeignKeys[0].ToColumn)
}

func TestGenerateMermaidER(t *testing.T) {
	schema := &DatabaseSchema{
		Tables: []Table{
			{
				Name: "users",
				Columns: []Column{
					{Name: "id", Type: "INTEGER", PK: true},
					{Name: "name", Type: "TEXT"},
				},
			},
			{
				Name: "orders",
				Columns: []Column{
					{Name: "id", Type: "INTEGER", PK: true},
					{Name: "user_id", Type: "INTEGER", FK: true},
				},
				ForeignKeys: []ForeignKey{
					{FromColumn: "user_id", ToTable: "users", ToColumn: "id"},
				},
			},
		},
	}

	mermaid := generateMermaidER(schema)
	assert.Contains(t, mermaid, "erDiagram")
	assert.Contains(t, mermaid, "users {")
	assert.Contains(t, mermaid, "orders {")
	assert.Contains(t, mermaid, "INTEGER id PK")
	assert.Contains(t, mermaid, "orders }o--|| users : \"user_id\"")
}

func TestSchemaCommand_Integration(t *testing.T) {
	// 1. Create temporary SQLite DB
	tmpDir, err := os.MkdirTemp("", "recac-schema-cmd-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)

	_, err = db.Exec("CREATE TABLE items (id INT, name TEXT);")
	require.NoError(t, err)
	db.Close()

	// 2. Run command via executeCommand helper
	// We use the helper from test_helpers_test.go if available, or just replicate the logic
	// Since executeCommand is unexported in test_helpers_test.go but shared in package main_test, it should be available if we are in package main

	// However, schema_test.go is in package main, but test_helpers_test.go defines `executeCommand`.
	// Let's rely on it being available.

	output, err := executeCommand(rootCmd, "schema", dbPath)
	require.NoError(t, err)

	assert.Contains(t, output, "erDiagram")
	assert.Contains(t, output, "items {")
	assert.Contains(t, output, "name")
}

func TestSchemaCommand_OutputFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-schema-out-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	outPath := filepath.Join(tmpDir, "schema.md")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE test (id INT);")
	require.NoError(t, err)
	db.Close()

	output, err := executeCommand(rootCmd, "schema", dbPath, "--output", outPath)
	require.NoError(t, err)

	assert.Contains(t, output, "Schema saved to")
	assert.Contains(t, output, outPath)

	content, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "```mermaid")
	assert.Contains(t, string(content), "test {")
}
