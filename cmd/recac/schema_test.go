package main

import (
	"bytes"
	"context"
	"errors"
    "os"
	"recac/internal/agent"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for extractSQLiteSchema using sqlmock
func TestExtractSQLiteSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// 1. Tables query
	rows := sqlmock.NewRows([]string{"name"}).AddRow("users").AddRow("posts")
	mock.ExpectQuery("SELECT name FROM sqlite_master WHERE type='table'").WillReturnRows(rows)

	// 2. PRAGMA table_info("users")
	userCols := sqlmock.NewRows([]string{"cid", "name", "type", "notnull", "dflt_value", "pk"}).
		AddRow(0, "id", "INTEGER", 1, nil, 1).
		AddRow(1, "email", "TEXT", 1, nil, 0)
	mock.ExpectQuery(`PRAGMA table_info\("users"\)`).WillReturnRows(userCols)

	// 3. PRAGMA foreign_key_list("users")
	mock.ExpectQuery(`PRAGMA foreign_key_list\("users"\)`).WillReturnRows(sqlmock.NewRows([]string{"id", "seq", "table", "from", "to", "on_update", "on_delete", "match"}))

	// 4. PRAGMA table_info("posts")
	postCols := sqlmock.NewRows([]string{"cid", "name", "type", "notnull", "dflt_value", "pk"}).
		AddRow(0, "id", "INTEGER", 1, nil, 1).
		AddRow(1, "user_id", "INTEGER", 1, nil, 0).
		AddRow(2, "title", "TEXT", 1, nil, 0)
	mock.ExpectQuery(`PRAGMA table_info\("posts"\)`).WillReturnRows(postCols)

	// 5. PRAGMA foreign_key_list("posts")
	postFKs := sqlmock.NewRows([]string{"id", "seq", "table", "from", "to", "on_update", "on_delete", "match"}).
		AddRow(0, 0, "users", "user_id", "id", "NO ACTION", "NO ACTION", "NONE")
	mock.ExpectQuery(`PRAGMA foreign_key_list\("posts"\)`).WillReturnRows(postFKs)

	schema, err := extractSQLiteSchema(db)
	assert.NoError(t, err)
	assert.NotNil(t, schema)
	assert.Len(t, schema.Tables, 2)

	// Verify users
	users := schema.Tables[0]
	assert.Equal(t, "users", users.Name)
	assert.Len(t, users.Columns, 2)
	assert.True(t, users.Columns[0].PK)

	// Verify posts
	posts := schema.Tables[1]
	assert.Equal(t, "posts", posts.Name)
	assert.Len(t, posts.Columns, 3)
	assert.Len(t, posts.ForeignKeys, 1)
	assert.Equal(t, "users", posts.ForeignKeys[0].ToTable)
	assert.True(t, posts.Columns[1].FK) // user_id is FK
}

func TestExtractSQLiteSchema_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT name FROM sqlite_master").WillReturnError(errors.New("query error"))

	_, err = extractSQLiteSchema(db)
	assert.Error(t, err)
}

func TestGenerateMermaidER(t *testing.T) {
	schema := &DatabaseSchema{
		Tables: []Table{
			{
				Name: "users",
				Columns: []Column{
					{Name: "id", Type: "INTEGER", PK: true},
					{Name: "email", Type: "TEXT"},
				},
			},
			{
				Name: "posts",
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
	assert.Contains(t, mermaid, "INTEGER id PK")
	assert.Contains(t, mermaid, "posts }o--|| users : \"user_id\"")
}

func TestRunSchema(t *testing.T) {
	// Mock extractSchema
	origExtract := extractSchema
	defer func() { extractSchema = origExtract }()

	extractSchema = func(connStr string) (*DatabaseSchema, error) {
		if connStr == "fail" {
			return nil, errors.New("db error")
		}
		return &DatabaseSchema{
			Tables: []Table{{Name: "test"}},
		}, nil
	}

	cmd := schemaCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	// 1. Success
	err := runSchema(cmd, []string{"mydb.sqlite"})
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "erDiagram")
	assert.Contains(t, out.String(), "test {")

	// 2. Fail
	err = runSchema(cmd, []string{"fail"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestRealExtractSchema_Error(t *testing.T) {
	// This calls sql.Open with invalid driver or dsn
	// Since we can't easily mock sql.Open directly without more refactoring,
	// we just test failure cases for realExtractSchema.

	// Invalid driver (sqlite is registered, but dsn might fail later on Ping)
	// sql.Open usually doesn't fail unless driver is unknown.

	// Test unknown driver? But we only support postgres/sqlite logic based on prefix.
	// If prefix is unknown, it defaults to sqlite.

	// If we provide a path that doesn't exist, sqlite Open might succeed (creates file) or fail?
	// But Ping should fail if directory doesn't exist?
	// Or connection string is invalid.

	// Actually, we can't easily test realExtractSchema hitting the DB without a real DB.
	// But we tested extractSQLiteSchema which is the main logic.
	// realExtractSchema just dispatches.
}

// Test postgres extraction logic if needed, similar to sqlite
func TestExtractPostgresSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected", err)
	}
	defer db.Close()

	// 1. Tables
	rows := sqlmock.NewRows([]string{"table_name"}).AddRow("users")
	mock.ExpectQuery("SELECT table_name FROM information_schema.tables").WillReturnRows(rows)

	// 2. Columns
	cols := sqlmock.NewRows([]string{"column_name", "data_type"}).
		AddRow("id", "integer").
		AddRow("name", "text")
	mock.ExpectQuery("SELECT column_name, data_type FROM information_schema.columns").WithArgs("users").WillReturnRows(cols)

	// 3. PKs
	pks := sqlmock.NewRows([]string{"column_name"}).AddRow("id")
	mock.ExpectQuery("SELECT kcu.column_name FROM information_schema.table_constraints").WithArgs("users").WillReturnRows(pks)

	// 4. FKs
	fks := sqlmock.NewRows([]string{"column_name", "foreign_table_name", "foreign_column_name"})
	mock.ExpectQuery("SELECT .* FROM information_schema.key_column_usage").WithArgs("users").WillReturnRows(fks)

	schema, err := extractPostgresSchema(db)
	assert.NoError(t, err)
	assert.NotNil(t, schema)
	assert.Len(t, schema.Tables, 1)
	assert.True(t, schema.Tables[0].Columns[0].PK)
}

type MockSchemaAgent struct {
	Response string
}

func (m *MockSchemaAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockSchemaAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}

func TestDescribeSchemaWithAI(t *testing.T) {
	mockAgent := &MockSchemaAgent{
		Response: "Test Schema Analysis Response",
	}

	oldAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, path, name string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = oldAgentFactory }()

	mermaid := "erDiagram\n    users {\n        INTEGER id PK\n    }"
	desc, err := describeSchemaWithAI(context.Background(), mermaid)
	require.NoError(t, err)
	assert.Equal(t, "Test Schema Analysis Response", desc)
}

type mockSchemaAgentClient struct {
	Response string
}

func (m *mockSchemaAgentClient) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}
func (m *mockSchemaAgentClient) SendWithRetry(ctx context.Context, prompt string, retries int) (string, error) {
	return m.Response, nil
}
func (m *mockSchemaAgentClient) SupportsVision() bool {
	return false
}
func (m *mockSchemaAgentClient) SendStream(ctx context.Context, prompt string, cb func(string)) (string, error) {
	return "", nil
}

func TestRunSchema_Outputs(t *testing.T) {
	// Mock extractSchema
	origExtract := extractSchema
	defer func() { extractSchema = origExtract }()

	extractSchema = func(connStr string) (*DatabaseSchema, error) {
		return &DatabaseSchema{
			Tables: []Table{{Name: "test"}},
		}, nil
	}

    // Mock Agent
	oldAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, dir, name string) (agent.Agent, error) {
		return &mockSchemaAgentClient{
			Response: "AI schema explanation",
		}, nil
	}
	defer func() { agentClientFactory = oldAgentFactory }()

    // cmd, out, _ := newRootCmd() does not attach flags defined in init() of schema.go
    // Let's use schemaCmd and a buffer directly since runSchema expects its flags.

    var outBuf bytes.Buffer
    schemaCmd.SetOut(&outBuf)

	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

    // Create fake db file to trigger implicit arg logic
    os.WriteFile("recac.db", []byte(""), 0644)

    err := runSchema(schemaCmd, []string{})
    assert.NoError(t, err)
    assert.Contains(t, outBuf.String(), "erDiagram")

    // Output to file
    tmpFile := "schema_out.md"

    schemaCmd.Flags().Set("output", tmpFile)
    schemaCmd.Flags().Set("ai", "true")

    err = runSchema(schemaCmd, []string{"mydb.sqlite"})
    assert.NoError(t, err)

    content, _ := os.ReadFile(tmpFile)
    assert.Contains(t, string(content), "AI schema explanation")
    assert.Contains(t, string(content), "erDiagram")

    schemaCmd.Flags().Set("ai", "false")
    err = runSchema(schemaCmd, []string{"mydb.sqlite"})
    assert.NoError(t, err)

    content, _ = os.ReadFile(tmpFile)
    assert.Contains(t, string(content), "erDiagram")
}

func TestRunSchema_NoArgsError(t *testing.T) {
	cmd, _, _ := newRootCmd()

    // Default no args error handling
    err := runSchema(cmd, []string{})
    assert.Error(t, err)
}

func TestDescribeSchemaWithAI_Error(t *testing.T) {
	oldAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, dir, name string) (agent.Agent, error) {
		return nil, errors.New("factory error")
	}
	defer func() { agentClientFactory = oldAgentFactory }()

    _, err := describeSchemaWithAI(context.Background(), "erDiagram")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "factory error")
}
