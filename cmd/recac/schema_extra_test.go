package main

import (
	"context"
	"testing"
	"recac/internal/agent"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestExtractPostgresSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// Mock queries
	// The query uses $1 placeholder which sqlmock handles?

	mock.ExpectQuery("SELECT table_name FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("users"))

	mock.ExpectQuery("SELECT column_name, data_type FROM information_schema.columns").
		WithArgs("users").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type"}).
			AddRow("id", "integer").
			AddRow("name", "text"))

	// PK query
	mock.ExpectQuery("SELECT kcu.column_name FROM information_schema.table_constraints").
		WithArgs("users").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}).AddRow("id"))

	// FK query
	mock.ExpectQuery("SELECT .* FROM information_schema.key_column_usage").
		WithArgs("users").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "foreign_table_name", "foreign_column_name"}))

	schema, err := extractPostgresSchema(db)
	assert.NoError(t, err)
	assert.Len(t, schema.Tables, 1)
	assert.Equal(t, "users", schema.Tables[0].Name)
	assert.Len(t, schema.Tables[0].Columns, 2)
	assert.True(t, schema.Tables[0].Columns[0].PK) // id should be PK
}

func TestDescribeSchemaWithAI(t *testing.T) {
	// Mock agent factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("This is a user schema")

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	desc, err := describeSchemaWithAI(context.Background(), "erDiagram")
	assert.NoError(t, err)
	assert.Equal(t, "This is a user schema", desc)
}
