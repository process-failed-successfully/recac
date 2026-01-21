package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

var schemaCmd = &cobra.Command{
	Use:   "schema [connection-string|file-path]",
	Short: "Reverse engineer database schema",
	Long: `Connects to a database (SQLite or Postgres) and generates a schema visualization.
Can output a Mermaid ER diagram and optionally use AI to document the domain model.

Examples:
  recac schema ./my.db
  recac schema "postgres://user:pass@localhost/dbname?sslmode=disable"
  recac schema ./my.db --ai --output schema.md`,
	RunE: runSchema,
}

func init() {
	rootCmd.AddCommand(schemaCmd)
	schemaCmd.Flags().StringP("output", "o", "", "Output file path (default stdout)")
	schemaCmd.Flags().Bool("ai", false, "Use AI to describe the schema")
}

type Column struct {
	Name string
	Type string
	PK   bool
	FK   bool
}

type ForeignKey struct {
	FromColumn string
	ToTable    string
	ToColumn   string
}

type Table struct {
	Name        string
	Columns     []Column
	ForeignKeys []ForeignKey
}

type DatabaseSchema struct {
	Tables []Table
}

func runSchema(cmd *cobra.Command, args []string) error {
	connStr := ""
	if len(args) > 0 {
		connStr = args[0]
	} else {
		// Try to find a default sqlite db
		if _, err := os.Stat("recac.db"); err == nil {
			connStr = "recac.db"
		} else if _, err := os.Stat(".recac.db"); err == nil {
			connStr = ".recac.db"
		} else {
			return fmt.Errorf("connection string or file path required")
		}
	}

	outputFile, _ := cmd.Flags().GetString("output")
	useAI, _ := cmd.Flags().GetBool("ai")

	// 1. Extract Schema
	schema, err := extractSchema(connStr)
	if err != nil {
		return fmt.Errorf("failed to extract schema: %w", err)
	}

	// 2. Generate Mermaid
	mermaid := generateMermaidER(schema)

	// 3. AI Analysis (if requested)
	var explanation string
	if useAI {
		explanation, err = describeSchemaWithAI(cmd.Context(), mermaid)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: AI analysis failed: %v\n", err)
		}
	}

	// 4. Output
	output := mermaid
	if explanation != "" {
		output = fmt.Sprintf("# Database Schema Documentation\n\n## Overview\n\n%s\n\n## Diagram\n\n```mermaid\n%s\n```\n", explanation, mermaid)
	} else if outputFile != "" && strings.HasSuffix(outputFile, ".md") {
		// Wrap in markdown block if writing to md file without AI
		output = fmt.Sprintf("```mermaid\n%s\n```\n", mermaid)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Schema saved to %s\n", outputFile)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), output)
	}

	return nil
}

func extractSchema(connStr string) (*DatabaseSchema, error) {
	var dbType string
	var dsn string

	if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		dbType = "postgres"
		dsn = connStr
	} else {
		// Assume SQLite file
		dbType = "sqlite"
		dsn = connStr
	}

	db, err := sql.Open(dbType, dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if dbType == "sqlite" {
		return extractSQLiteSchema(db)
	} else {
		return extractPostgresSchema(db)
	}
}

func extractSQLiteSchema(db *sql.DB) (*DatabaseSchema, error) {
	schema := &DatabaseSchema{}

	// Get tables
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}

	for _, tableName := range tableNames {
		table := Table{Name: tableName}

		// Get columns
		// Escape quotes in table name
		safeTableName := strings.ReplaceAll(tableName, "\"", "\"\"")
		colRows, err := db.Query(fmt.Sprintf("PRAGMA table_info(\"%s\")", safeTableName))
		if err != nil {
			return nil, err
		}

		pkSet := make(map[string]bool)

		for colRows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dfltValue interface{}

			if err := colRows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
				colRows.Close()
				return nil, err
			}

			isPK := pk > 0
			if isPK {
				pkSet[name] = true
			}

			table.Columns = append(table.Columns, Column{
				Name: name,
				Type: ctype,
				PK:   isPK,
			})
		}
		colRows.Close()

		// Get Foreign Keys
		fkRows, err := db.Query(fmt.Sprintf("PRAGMA foreign_key_list(\"%s\")", safeTableName))
		if err != nil {
			return nil, err
		}

		for fkRows.Next() {
			var id, seq int
			var tableStr, from, to, onUpdate, onDelete, match string
			if err := fkRows.Scan(&id, &seq, &tableStr, &from, &to, &onUpdate, &onDelete, &match); err != nil {
				fkRows.Close()
				return nil, err
			}

			table.ForeignKeys = append(table.ForeignKeys, ForeignKey{
				FromColumn: from,
				ToTable:    tableStr,
				ToColumn:   to,
			})

			// Mark column as FK
			for i, c := range table.Columns {
				if c.Name == from {
					table.Columns[i].FK = true
				}
			}
		}
		fkRows.Close()

		schema.Tables = append(schema.Tables, table)
	}

	return schema, nil
}

func extractPostgresSchema(db *sql.DB) (*DatabaseSchema, error) {
	schema := &DatabaseSchema{}

	// Get tables
	rows, err := db.Query(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}

	for _, tableName := range tableNames {
		table := Table{Name: tableName}

		// Get columns
		colQuery := `
			SELECT column_name, data_type
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
		`
		colRows, err := db.Query(colQuery, tableName)
		if err != nil {
			return nil, err
		}

		for colRows.Next() {
			var name, ctype string
			if err := colRows.Scan(&name, &ctype); err != nil {
				colRows.Close()
				return nil, err
			}
			table.Columns = append(table.Columns, Column{
				Name: name,
				Type: ctype,
			})
		}
		colRows.Close()

		// Get PKs
		pkQuery := `
			SELECT kcu.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
			  ON tc.constraint_name = kcu.constraint_name
			  AND tc.table_schema = kcu.table_schema
			WHERE tc.constraint_type = 'PRIMARY KEY' AND tc.table_name = $1
		`
		pkRows, err := db.Query(pkQuery, tableName)
		if err != nil {
			return nil, err
		}
		pkSet := make(map[string]bool)
		for pkRows.Next() {
			var colName string
			if err := pkRows.Scan(&colName); err == nil {
				pkSet[colName] = true
			}
		}
		pkRows.Close()

		// Apply PKs
		for i, c := range table.Columns {
			if pkSet[c.Name] {
				table.Columns[i].PK = true
			}
		}

		// Get FKs
		fkQuery := `
			SELECT
				kcu.column_name,
				ccu.table_name AS foreign_table_name,
				ccu.column_name AS foreign_column_name
			FROM
				information_schema.key_column_usage AS kcu
			JOIN
				information_schema.constraint_column_usage AS ccu
				ON ccu.constraint_name = kcu.constraint_name
			JOIN
				information_schema.table_constraints AS tc
				ON tc.constraint_name = kcu.constraint_name
			WHERE constraint_type = 'FOREIGN KEY' AND tc.table_name = $1
		`
		fkRows, err := db.Query(fkQuery, tableName)
		if err != nil {
			return nil, err
		}

		for fkRows.Next() {
			var from, toTable, toCol string
			if err := fkRows.Scan(&from, &toTable, &toCol); err != nil {
				fkRows.Close()
				return nil, err
			}
			table.ForeignKeys = append(table.ForeignKeys, ForeignKey{
				FromColumn: from,
				ToTable:    toTable,
				ToColumn:   toCol,
			})

			// Mark as FK
			for i, c := range table.Columns {
				if c.Name == from {
					table.Columns[i].FK = true
				}
			}
		}
		fkRows.Close()

		schema.Tables = append(schema.Tables, table)
	}

	return schema, nil
}

func generateMermaidER(schema *DatabaseSchema) string {
	var sb strings.Builder
	sb.WriteString("erDiagram\n")

	for _, t := range schema.Tables {
		sb.WriteString(fmt.Sprintf("    %s {\n", t.Name))
		for _, c := range t.Columns {
			keyType := ""
			if c.PK && c.FK {
				keyType = "PK,FK"
			} else if c.PK {
				keyType = "PK"
			} else if c.FK {
				keyType = "FK"
			}

			// Mermaid format: Type Name Key
			// Replace spaces in type with underscores or quotes if needed
			safeType := strings.ReplaceAll(c.Type, " ", "_")
			sb.WriteString(fmt.Sprintf("        %s %s %s\n", safeType, c.Name, keyType))
		}
		sb.WriteString("    }\n")
	}

	for _, t := range schema.Tables {
		for _, fk := range t.ForeignKeys {
			// Relationship: From }|..|| To (simplified)
			// We often don't know cardinality from just FK, so assume one-to-many or zero-to-many
			// FK table has the "many" side usually.
			sb.WriteString(fmt.Sprintf("    %s }o--|| %s : \"%s\"\n", t.Name, fk.ToTable, fk.FromColumn))
		}
	}

	return sb.String()
}

func describeSchemaWithAI(ctx context.Context, mermaid string) (string, error) {
	cwd, _ := os.Getwd()
	ag, err := agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), cwd, "recac-schema")
	if err != nil {
		return "", err
	}

	prompt := fmt.Sprintf(`Analyze the following Database Schema (Mermaid ER Diagram) and provide a high-level documentation.
Describe the core entities, their relationships, and the likely business domain.
Identify any potential design issues (e.g. missing foreign keys, odd naming).

Schema:
'''
%s
'''`, mermaid)

	return ag.Send(ctx, prompt)
}
