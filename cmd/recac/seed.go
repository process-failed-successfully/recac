package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

var (
	seedDbPath  string
	seedRows    int
	seedExecute bool
	seedOutput  string
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Generate synthetic data for database seeding",
	Long: `Analyze your database schema and generate realistic synthetic data using AI.
Can output SQL INSERT statements or execute them directly.

Examples:
  recac seed --db ./my.db --rows 10 --execute
  recac seed --db "postgres://user:pass@localhost/dbname" --output data.sql
`,
	RunE: runSeed,
}

func init() {
	rootCmd.AddCommand(seedCmd)
	seedCmd.Flags().StringVarP(&seedDbPath, "db", "d", "", "Database connection string or file path")
	seedCmd.Flags().IntVarP(&seedRows, "rows", "r", 10, "Number of rows to generate per table")
	seedCmd.Flags().BoolVarP(&seedExecute, "execute", "x", false, "Execute the generated SQL immediately")
	seedCmd.Flags().StringVarP(&seedOutput, "output", "o", "", "Output file path for the SQL (default stdout)")
}

func runSeed(cmd *cobra.Command, args []string) error {
	// 1. Resolve DB Connection
	connStr := seedDbPath
	if connStr == "" {
		// Try to find a default sqlite db
		if _, err := os.Stat("recac.db"); err == nil {
			connStr = "recac.db"
		} else if _, err := os.Stat(".recac.db"); err == nil {
			connStr = ".recac.db"
		} else {
			return fmt.Errorf("connection string or file path required (use --db)")
		}
	}

	// 2. Extract Schema
	// reuse extractSchema from schema.go
	schema, err := extractSchema(connStr)
	if err != nil {
		return fmt.Errorf("failed to extract schema: %w", err)
	}

	// 3. Prepare AI Prompt
	ddl := schemaToDDL(schema)

	ctx := cmd.Context()
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

	prompt := fmt.Sprintf(`You are a Database Expert.
Given the following database schema, generate valid SQL INSERT statements to seed the database with realistic test data.
Generate approximately %d rows for each table.
Ensure Foreign Key constraints are satisfied (create parent records before child records).
Return ONLY the raw SQL queries. Do not use Markdown formatting (no code blocks).

Schema:
%s
`, seedRows, ddl)

	fmt.Fprintln(cmd.ErrOrStderr(), "🤖 Generating synthetic data...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	sqlQuery := utils.CleanCodeBlock(resp)

	// 4. Output SQL
	if seedOutput != "" {
		if err := os.WriteFile(seedOutput, []byte(sqlQuery), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ Seed data saved to %s\n", seedOutput)
	} else {
		if !seedExecute {
			fmt.Fprintf(cmd.OutOrStdout(), "-- Generated Seed Data:\n%s\n\n", sqlQuery)
		}
	}

	// 5. Execute (if requested)
	if seedExecute {
		fmt.Fprintln(cmd.ErrOrStderr(), "🚀 Executing seed data...")
		if err := executeSeedSQL(connStr, sqlQuery); err != nil {
			return fmt.Errorf("execution failed: %w", err)
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "✅ Database seeded successfully!")
	}

	return nil
}

func executeSeedSQL(connStr, query string) error {
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
		return err
	}
	defer tx.Rollback()

	// Execute the entire script
	// Note: Some drivers might not support multiple statements in one Exec.
	// However, modernc.org/sqlite and lib/pq generally do for scripts.
	// If this fails, we might need to split by semicolon, but that's fragile with strings.
	_, err = tx.Exec(query)
	if err != nil {
		return err
	}

	return tx.Commit()
}
