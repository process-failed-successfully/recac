package main

import (
	"context"
	"fmt"
	"os"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	seedDbStr   string
	seedRows    int
	seedExecute bool
	seedOutput  string
	seedClean   bool
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Generate realistic database seed data using AI",
	Long: `Analyzes your database schema and generates realistic INSERT statements to populate it with dummy data.
It understands foreign key relationships and ensures referential integrity.

Examples:
  recac seed --db recac.db --rows 20
  recac seed --db "postgres://..." --clean --execute
`,
	RunE: runSeed,
}

func init() {
	rootCmd.AddCommand(seedCmd)
	seedCmd.Flags().StringVarP(&seedDbStr, "db", "d", "", "Database connection string or file path")
	seedCmd.Flags().IntVarP(&seedRows, "rows", "r", 10, "Number of rows to generate per table")
	seedCmd.Flags().BoolVarP(&seedExecute, "execute", "x", false, "Execute the generated SQL immediately")
	seedCmd.Flags().StringVarP(&seedOutput, "output", "o", "", "Output file path for the SQL")
	seedCmd.Flags().BoolVar(&seedClean, "clean", false, "Include TRUNCATE/DELETE statements to clean tables before seeding")
}

func runSeed(cmd *cobra.Command, args []string) error {
	// 1. Resolve DB Connection
	connStr := seedDbStr
	if connStr == "" {
		// Try defaults
		if _, err := os.Stat("recac.db"); err == nil {
			connStr = "recac.db"
		} else if _, err := os.Stat(".recac.db"); err == nil {
			connStr = ".recac.db"
		} else {
			return fmt.Errorf("connection string or file path required (use --db)")
		}
	}

	// 2. Extract Schema
	// extractSchema is defined in schema.go (package main)
	schema, err := extractSchema(connStr)
	if err != nil {
		return fmt.Errorf("failed to extract schema: %w", err)
	}

	if len(schema.Tables) == 0 {
		return fmt.Errorf("no tables found in database")
	}

	// 3. Prepare AI Prompt
	// schemaToDDL is defined in sql.go (package main)
	ddl := schemaToDDL(schema)

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

	cleanInstruction := ""
	if seedClean {
		cleanInstruction = "Include SQL statements to TRUNCATE or DELETE all data from the tables first (handle foreign key constraints if needed, e.g. CASCADE or order of deletion)."
	}

	prompt := fmt.Sprintf(`You are an expert Database Administrator and QA Engineer.
Your task is to generate realistic seed data for the following database schema.

Schema:
%s

Requirements:
1. Generate approximately %d rows for EACH table.
2. Ensure Referential Integrity: Foreign keys must point to valid existing IDs generated in previous steps.
3. Order of Insertion: Insert data into independent tables first, then dependent tables.
4. Data Quality: Use realistic names, emails, dates, and text.
5. Format: Return ONLY valid SQL INSERT statements.
6. %s

Output:
Provide ONLY the raw SQL. Do not use Markdown blocks.
`, ddl, seedRows, cleanInstruction)

	fmt.Fprintf(cmd.ErrOrStderr(), "🤖 Generating seed data for %d tables (%d rows each)...\n", len(schema.Tables), seedRows)

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
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ SQL saved to %s\n", seedOutput)
	} else {
		if !seedExecute {
			fmt.Fprintf(cmd.OutOrStdout(), "-- Generated Seed Data:\n%s\n", sqlQuery)
		}
	}

	// 5. Execute
	if seedExecute {
		fmt.Fprintln(cmd.ErrOrStderr(), "🚀 Executing SQL...")
		// executeSQL is defined in sql.go (package main)
		if err := executeSQL(cmd, connStr, sqlQuery); err != nil {
			return fmt.Errorf("execution failed: %w", err)
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "✅ Seed execution completed.")
	}

	return nil
}
