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
	seedExecute bool
	seedOutput  string
	seedCount   int
	seedClean   bool
	seedTables  []string
)

var seedCmd = &cobra.Command{
	Use:   "seed [connection-string]",
	Short: "Generate realistic seed data for your database",
	Long: `Analyze your database schema and generate realistic seed data (SQL INSERT statements) using AI.
Can automatically execute the SQL to populate your database.

Examples:
  recac seed ./my.db --count 50 --execute
  recac seed "postgres://..." --tables users,orders --clean
`,
	RunE: runSeed,
}

func init() {
	rootCmd.AddCommand(seedCmd)
	seedCmd.Flags().StringVarP(&seedDbPath, "db", "d", "", "Database connection string or file path")
	seedCmd.Flags().BoolVarP(&seedExecute, "execute", "x", false, "Execute the generated SQL query")
	seedCmd.Flags().StringVarP(&seedOutput, "output", "o", "", "Output file path for the SQL (default stdout)")
	seedCmd.Flags().IntVarP(&seedCount, "count", "n", 10, "Approximate number of rows per table")
	seedCmd.Flags().BoolVar(&seedClean, "clean", false, "Generate DELETE/TRUNCATE statements before inserting")
	seedCmd.Flags().StringSliceVar(&seedTables, "tables", nil, "Specific tables to seed (comma separated)")
}

func runSeed(cmd *cobra.Command, args []string) error {
	// 1. Resolve DB Connection
	connStr := seedDbPath
	if len(args) > 0 {
		connStr = args[0]
	}

	if connStr == "" {
		// Try to find a default sqlite db
		if _, err := os.Stat("recac.db"); err == nil {
			connStr = "recac.db"
		} else if _, err := os.Stat(".recac.db"); err == nil {
			connStr = ".recac.db"
		} else {
			return fmt.Errorf("connection string or file path required (use --db or argument)")
		}
	}

	// 2. Extract Schema
	fmt.Fprintf(cmd.ErrOrStderr(), "🔍 Analyzing schema from %s...\n", connStr)
	schema, err := extractSchema(connStr)
	if err != nil {
		return fmt.Errorf("failed to extract schema: %w", err)
	}

	// 3. Filter Tables
	if len(seedTables) > 0 {
		var filtered []Table
		include := make(map[string]bool)
		for _, t := range seedTables {
			include[t] = true
		}
		for _, t := range schema.Tables {
			if include[t.Name] {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("no matching tables found for: %v", seedTables)
		}
		schema.Tables = filtered
	}

	// 4. Prepare AI Prompt
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

	cleanInstruction := ""
	if seedClean {
		cleanInstruction = "- Include statements to DELETE or TRUNCATE the tables before inserting."
	}

	prompt := fmt.Sprintf(`You are an expert Database Administrator.
Generate SQL INSERT statements to populate the following database tables with realistic test data.
The data must be consistent (respect Foreign Key constraints).

Schema:
%s

Requirements:
- Generate approximately %d rows for each table.
- Use realistic data (names, emails, dates, addresses).
- %s
- If there are dependencies, insert into parent tables first.
- Return ONLY the raw SQL commands. Do not wrap in markdown code blocks.
`, ddl, seedCount, cleanInstruction)

	fmt.Fprintln(cmd.ErrOrStderr(), "🤖 Generating seed data with AI...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	sqlQuery := utils.CleanCodeBlock(resp)

	// 5. Output SQL
	if seedOutput != "" {
		if err := os.WriteFile(seedOutput, []byte(sqlQuery), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ SQL saved to %s\n", seedOutput)
	} else {
		if !seedExecute {
			fmt.Fprintln(cmd.OutOrStdout(), sqlQuery)
		}
	}

	// 6. Execute (if requested)
	if seedExecute {
		fmt.Fprintln(cmd.ErrOrStderr(), "🚀 Executing SQL...")

		if err := executeSeedSQL(cmd, connStr, sqlQuery); err != nil {
			return err
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "✅ Database seeded successfully.")
	}

	return nil
}

// executeSeedSQL executes SQL that doesn't necessarily return rows (like INSERT)
// It supports multiple statements by wrapping them in a transaction if possible,
// or just executing the block.
func executeSeedSQL(cmd *cobra.Command, connStr, query string) error {
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

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Execute the query (script)
	// modernc.org/sqlite supports executing a script via Exec if it contains multiple statements.
	// However, standard database/sql Exec usually prepares.
	// If it fails, we might need to split.
	_, err = tx.Exec(query)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute SQL script: %w", err)
	}

	return tx.Commit()
}
