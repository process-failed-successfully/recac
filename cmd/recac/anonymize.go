package main

import (
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
	anonymizeDbPath  string
	anonymizeExecute bool
	anonymizeOutput  string
)

var anonymizeCmd = &cobra.Command{
	Use:   "anonymize",
	Short: "Anonymize sensitive data in a database",
	Long: `Analyze your database schema to identify Personally Identifiable Information (PII).
Generates and executes SQL UPDATE statements to replace sensitive data with realistic, anonymized values.
Preserves Referential Integrity and Uniqueness constraints where possible.

Examples:
  recac anonymize --db ./production_dump.db --output anonymize.sql
  recac anonymize --db "postgres://user:pass@localhost/dbname" --execute
`,
	RunE: runAnonymize,
}

func init() {
	rootCmd.AddCommand(anonymizeCmd)
	anonymizeCmd.Flags().StringVarP(&anonymizeDbPath, "db", "d", "", "Database connection string or file path")
	anonymizeCmd.Flags().BoolVarP(&anonymizeExecute, "execute", "x", false, "Execute the generated SQL immediately")
	anonymizeCmd.Flags().StringVarP(&anonymizeOutput, "output", "o", "", "Output file path for the SQL (default stdout)")
}

func runAnonymize(cmd *cobra.Command, args []string) error {
	// 1. Resolve DB Connection
	connStr := anonymizeDbPath
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
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-anonymize")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Determine DB type for SQL syntax context
	dbType := "SQLite"
	if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		dbType = "PostgreSQL"
	}

	prompt := fmt.Sprintf(`You are a Data Privacy and Database Expert.
Your task is to anonymize sensitive Personally Identifiable Information (PII) in the given database schema.

1. Identify columns containing PII (e.g., names, emails, phones, addresses, SSN, credit cards).
2. Generate a SQL script to UPDATE these columns with anonymized, synthetic data.
3. Use deterministic logic based on the Primary Key (if available) to ensure uniqueness for unique columns.
   - Example for Email: 'user_' || id || '@anon.com'
   - Example for Name: 'User ' || id
   - Example for Phone: '555-' || (10000 + id)
4. Do NOT modify Primary Keys or Foreign Keys.
5. Use SQL syntax valid for %s.
6. Return ONLY the raw SQL queries. Do not use Markdown formatting (no code blocks).

Schema:
%s
`, dbType, ddl)

	fmt.Fprintln(cmd.ErrOrStderr(), "🤖 Analyzing schema and generating anonymization script...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	sqlQuery := utils.CleanCodeBlock(resp)

	// 4. Output SQL
	if anonymizeOutput != "" {
		if err := os.WriteFile(anonymizeOutput, []byte(sqlQuery), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ Anonymization script saved to %s\n", anonymizeOutput)
	} else {
		if !anonymizeExecute {
			fmt.Fprintf(cmd.OutOrStdout(), "-- Generated Anonymization Script:\n%s\n\n", sqlQuery)
		}
	}

	// 5. Execute (if requested)
	if anonymizeExecute {
		fmt.Fprintln(cmd.ErrOrStderr(), "🚀 Executing anonymization script...")
		if err := executeSQLScript(connStr, sqlQuery); err != nil {
			return fmt.Errorf("execution failed: %w", err)
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "✅ Database anonymized successfully!")
	}

	return nil
}
