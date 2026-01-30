package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	seedExecute bool
	seedCount   int
	seedOutput  string
)

var seedCmd = &cobra.Command{
	Use:   "seed [connection-string|file-path]",
	Short: "Populate database with AI-generated dummy data",
	Long: `Analyzes the database schema and uses AI to generate realistic dummy data (INSERT statements).
It respects foreign key constraints and data types.

Examples:
  recac seed ./my.db --count 10 --execute
  recac seed "postgres://..." --output seed.sql`,
	RunE: runSeed,
}

func init() {
	rootCmd.AddCommand(seedCmd)
	seedCmd.Flags().BoolVarP(&seedExecute, "execute", "x", false, "Execute the generated SQL immediately")
	seedCmd.Flags().IntVarP(&seedCount, "count", "c", 5, "Number of rows to generate per table")
	seedCmd.Flags().StringVarP(&seedOutput, "output", "o", "", "Output file for SQL statements")
}

func runSeed(cmd *cobra.Command, args []string) error {
	connStr := ""
	if len(args) > 0 {
		connStr = args[0]
	} else {
		// Try to find a default sqlite db (reusing logic from schema.go/sql.go)
		if _, err := os.Stat("recac.db"); err == nil {
			connStr = "recac.db"
		} else if _, err := os.Stat(".recac.db"); err == nil {
			connStr = ".recac.db"
		} else {
			return fmt.Errorf("connection string or file path required")
		}
	}

	// 1. Extract Schema (reusing function from schema.go)
	schema, err := extractSchema(connStr)
	if err != nil {
		return fmt.Errorf("failed to extract schema: %w", err)
	}

	// 2. Prepare Agent
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-seed")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// 3. Construct Prompt
	ddl := seedSchemaToDDL(schema)
	prompt := fmt.Sprintf(`You are a Database Expert.
Generate valid SQL INSERT statements to populate the following database with realistic dummy data.
Generate exactly %d rows for EACH table.
Respect Foreign Key constraints: ensure referenced IDs exist (or use placeholders/variables if supported, but plain SQL is better).
Handle data types correctly (quotes for strings, integers for ints, 'YYYY-MM-DD' for dates).

Schema:
%s

IMPORTANT: Return ONLY the raw SQL statements. Do not use Markdown formatting (no code blocks).
Start with 'BEGIN;' and end with 'COMMIT;' if possible.
`, seedCount, ddl)

	fmt.Fprintln(cmd.ErrOrStderr(), "🤖 Generating seed data...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	sqlContent := utils.CleanCodeBlock(resp)

	// 4. Output
	if seedOutput != "" {
		if err := os.WriteFile(seedOutput, []byte(sqlContent), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ Seed SQL saved to %s\n", seedOutput)
	} else if !seedExecute {
		fmt.Fprintln(cmd.OutOrStdout(), sqlContent)
	}

	// 5. Execute
	if seedExecute {
		fmt.Fprintln(cmd.ErrOrStderr(), "🚀 Executing SQL...")
		if err := executeSeedSQL(cmd, connStr, sqlContent); err != nil {
			return fmt.Errorf("execution failed: %w", err)
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "✅ Data seeded successfully!")
	}

	return nil
}

// seedSchemaToDDL is a helper to convert schema to text for the prompt.
// Copied/Adapted from sql.go logic since it's unexported there.
func seedSchemaToDDL(schema *DatabaseSchema) string {
	var sb strings.Builder
	for _, t := range schema.Tables {
		sb.WriteString(fmt.Sprintf("TABLE %s (\n", t.Name))
		for _, c := range t.Columns {
			sb.WriteString(fmt.Sprintf("  %s %s", c.Name, c.Type))
			if c.PK {
				sb.WriteString(" PRIMARY KEY")
			}
			sb.WriteString(",\n")
		}
		for _, fk := range t.ForeignKeys {
			sb.WriteString(fmt.Sprintf("  FOREIGN KEY (%s) REFERENCES %s(%s),\n", fk.FromColumn, fk.ToTable, fk.ToColumn))
		}
		sb.WriteString(");\n")
	}
	return sb.String()
}

func executeSeedSQL(cmd *cobra.Command, connStr, sqlContent string) error {
	var dbType string
	var dsn string

	if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		dbType = "postgres"
		dsn = connStr
	} else {
		dbType = "sqlite"
		dsn = connStr
	}

	db, err := sql.Open(dbType, dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if not committed

	if _, err := tx.Exec(sqlContent); err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	return tx.Commit()
}
